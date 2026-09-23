package main

import (
	"crypto/ecdsa"
	cryptorand "crypto/rand"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type User struct { ID, Email, FirstName, LastName string }
type Member struct { User User `json:"user"`; Role string `json:"role"`; CanInvite bool `json:"canInvite"`; AllowMultipleDevices bool `json:"allowMultipleDevices"`; JoinedAt int64 `json:"joinedAt"` }
type Household struct { ID, Name string; Members []Member `json:"members"`; CreatedAt, UpdatedAt int64 }
// Quantity/Size/Unit are optional measures; zero and empty mean the user gave none.
type Item struct { ID, Name string; Checked bool; Quantity int; Size float64; Unit string; UpdatedAt int64 }
// EveryonePermission is "" when the list is not shared with everyone, otherwise the level
// ("view"/"check"/"edit") a household member gets by subscribing to it. It is owner-set
// through updatePermissions alongside the per-member Permissions map, and only takes effect
// once a member actually subscribes -- it does not by itself grant anyone access.
type List struct { ID, HouseholdID, OwnerID, Title, Icon, StoreCountryCode, StoreCountryName string; Items []Item; Permissions map[string]string `json:"permissions"`; EveryonePermission string; ItemHistory map[string]int `json:"itemHistory"`; SuggestionEvents map[string]int64 `json:"suggestionEvents,omitempty"`; QuickAddRows, MaxQuickSuggestions int; SuggestionIncludesMeasures bool; UpdatedAt int64; NudgedAt int64; NudgedBy string }
// Invite is a single-use, expiring admission ticket for one household. It replaces
// handing the long-lived master pairing key to every family member.
// Subject empty means "admit a new member". Subject set to a user id means "add another
// device for that same person", which is what stops a second phone becoming a second member.
type Invite struct { Code, HouseholdID, CreatedBy, UsedBy, Subject string; CreatedAt, ExpiresAt, UsedAt int64 }
// Device is an enrolled device. Only the public half of its key is ever here; the private
// half stays in the phone's keystore and cannot be exported.
type Device struct { ID, UserID, Name, X, Y string; EnrolledAt, LastSeenAt int64 }
// Backup is one device backup, kept per user. Backups are opaque to the connector: it
// stores and returns the document without interpreting it.
type Backup struct { ID, Name, Payload string; CreatedAt int64; Size int }
type State struct { Households map[string]Household `json:"households"`; Lists map[string]List `json:"lists"`; DeletedLists map[string]int64 `json:"deletedLists,omitempty"`; KnownUsers map[string]User `json:"knownUsers"`; Invites map[string]Invite `json:"invites"`; Backups map[string][]Backup `json:"backups"`; Devices map[string]Device `json:"devices"`; RecoveryCodes map[string]string `json:"recoveryCodes,omitempty"` }
type Store struct { sync.RWMutex; path string; State }

func newStore(path string) *Store {
	s := &Store{path: path, State: State{Households: map[string]Household{}, Lists: map[string]List{}, KnownUsers: map[string]User{}}}
	b, err := os.ReadFile(path); if err == nil { _ = json.Unmarshal(b, &s.State) }
	if s.Households==nil{s.Households=map[string]Household{}};if s.Lists==nil{s.Lists=map[string]List{}};if s.DeletedLists==nil{s.DeletedLists=map[string]int64{}};if s.KnownUsers==nil{s.KnownUsers=map[string]User{}};if s.Invites==nil{s.Invites=map[string]Invite{}};if s.Backups==nil{s.Backups=map[string][]Backup{}};if s.Devices==nil{s.Devices=map[string]Device{}};if s.RecoveryCodes==nil{s.RecoveryCodes=map[string]string{}}
	return s
}
func (s *Store) save() error {
	b, err := json.MarshalIndent(s.State, "", "  "); if err != nil { return err }
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil { return err }
	tmp := s.path+".tmp"; if err = os.WriteFile(tmp,b,0600); err != nil{return err}; return os.Rename(tmp,s.path)
}

type Claims struct { Sub, Email, GivenName, FamilyName, Name, Iss string; Aud any; Exp int64 }
// Auth validates provider-issued JWT access tokens. The issuer and JWKS location
// come from the provider's OIDC discovery document rather than being assumed, so
// any standards-compliant provider works, not only Authentik.
type Auth struct { configURL, audience string; mu sync.RWMutex; issuer, jwksURL string; keys map[string]any; fetched time.Time }
func newAuth(issuer,audience string)*Auth{return &Auth{configURL:strings.TrimRight(issuer,"/")+"/.well-known/openid-configuration",audience:audience,keys:map[string]any{}}}
func (a *Auth) discover()error{
	a.mu.RLock();loaded:=a.jwksURL!="";a.mu.RUnlock();if loaded{return nil}
	resp,e:=http.Get(a.configURL);if e!=nil{return e};defer resp.Body.Close()
	if resp.StatusCode!=200{return fmt.Errorf("discovery returned %s",resp.Status)}
	var doc struct{Issuer string `json:"issuer"`;JwksURI string `json:"jwks_uri"`}
	if e=json.NewDecoder(resp.Body).Decode(&doc);e!=nil{return e}
	if doc.Issuer==""||doc.JwksURI==""{return errors.New("discovery document is missing issuer or jwks_uri")}
	a.mu.Lock();a.issuer=doc.Issuer;a.jwksURL=doc.JwksURI;a.mu.Unlock();return nil
}
func (a *Auth) claims(r *http.Request)(Claims,error){
	var c Claims; h:=strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"),"Bearer ")); if h=="" { return c,errors.New("missing bearer token") }
	p:=strings.Split(h,"."); if len(p)!=3{return c,errors.New("invalid token")}; var head map[string]any
	if b,e:=base64.RawURLEncoding.DecodeString(p[0]);e!=nil{return c,e}else if e=json.Unmarshal(b,&head);e!=nil{return c,e}
	alg,_:=head["alg"].(string);if alg!="RS256"&&alg!="ES256"{return c,errors.New("token must use RS256 or ES256")}
	if e:=a.discover();e!=nil{return c,e}
	kid,_:=head["kid"].(string); key,e:=a.key(kid);if e!=nil{return c,e}
	sig,e:=base64.RawURLEncoding.DecodeString(p[2]);if e!=nil{return c,e}; sum:=sha256.Sum256([]byte(p[0]+"."+p[1]));if e=verifySignature(key,alg,sum[:],sig);e!=nil{return c,e}
	b,e:=base64.RawURLEncoding.DecodeString(p[1]);if e!=nil{return c,e};var raw map[string]any;if e=json.Unmarshal(b,&raw);e!=nil{return c,e}
	c.Sub=str(raw["sub"]);c.Email=str(raw["email"]);c.GivenName=str(raw["given_name"]);c.FamilyName=str(raw["family_name"]);c.Name=str(raw["name"]);c.Iss=str(raw["iss"]);c.Aud=raw["aud"];c.Exp=int64(number(raw["exp"]))
	a.mu.RLock();wantIssuer:=a.issuer;a.mu.RUnlock()
	if c.Sub==""||c.Iss!=wantIssuer||time.Now().Unix()>=c.Exp||!audienceOK(c.Aud,a.audience){return c,errors.New("token claims rejected")};return c,nil
}
func verifySignature(key any,alg string,sum,sig []byte)error{
	switch k:=key.(type){
	case *rsa.PublicKey:
		if alg!="RS256"{return errors.New("key does not match token algorithm")}
		if e:=rsa.VerifyPKCS1v15(k,cryptoHashSHA256,sum,sig);e!=nil{return errors.New("invalid signature")}
	case *ecdsa.PublicKey:
		if alg!="ES256"{return errors.New("key does not match token algorithm")}
		if len(sig)!=64{return errors.New("invalid signature")}
		if !ecdsa.Verify(k,sum,new(big.Int).SetBytes(sig[:32]),new(big.Int).SetBytes(sig[32:])){return errors.New("invalid signature")}
	default:
		return errors.New("unsupported signing key type")
	}
	return nil
}
// crypto.Hash(5) is SHA-256; spelling it numerically keeps this tiny service dependency-free.
const cryptoHashSHA256 = 5
func (a *Auth) key(kid string)(any,error){a.mu.RLock();k:=a.keys[kid];fresh:=time.Since(a.fetched)<time.Hour;a.mu.RUnlock();if k!=nil&&fresh{return k,nil};if e:=a.refresh();e!=nil{return nil,e};a.mu.RLock();defer a.mu.RUnlock();k=a.keys[kid];if k==nil{return nil,errors.New("unknown signing key")};return k,nil}
func (a *Auth) refresh()error{
	a.mu.RLock();url:=a.jwksURL;a.mu.RUnlock();if url==""{return errors.New("provider metadata is not loaded")}
	resp,e:=http.Get(url);if e!=nil{return e};defer resp.Body.Close()
	if resp.StatusCode!=200{return fmt.Errorf("jwks returned %s",resp.Status)}
	var doc struct{Keys []struct{Kid,Kty,Crv,N,E,X,Y string;X5C []string `json:"x5c"`} `json:"keys"`}
	if e=json.NewDecoder(resp.Body).Decode(&doc);e!=nil{return e}
	next:=map[string]any{}
	for _,j:=range doc.Keys{
		var k any
		switch{
		case len(j.X5C)>0:
			if der,e2:=base64.StdEncoding.DecodeString(j.X5C[0]);e2==nil{if cert,e3:=x509.ParseCertificate(der);e3==nil{k=cert.PublicKey}}
		case j.Kty=="EC":
			var curve elliptic.Curve
			switch j.Crv{case "P-256":curve=elliptic.P256();case "P-384":curve=elliptic.P384();case "P-521":curve=elliptic.P521()}
			if curve!=nil{xb,_:=base64.RawURLEncoding.DecodeString(j.X);yb,_:=base64.RawURLEncoding.DecodeString(j.Y);if len(xb)>0&&len(yb)>0{k=&ecdsa.PublicKey{Curve:curve,X:new(big.Int).SetBytes(xb),Y:new(big.Int).SetBytes(yb)}}}
		default:
			nb,_:=base64.RawURLEncoding.DecodeString(j.N);eb,_:=base64.RawURLEncoding.DecodeString(j.E);ei:=0;for _,v:=range eb{ei=ei*256+int(v)}
			if len(nb)>0&&ei>0{k=&rsa.PublicKey{N:new(big.Int).SetBytes(nb),E:ei}}
		}
		if k!=nil&&j.Kid!=""{next[j.Kid]=k}
	}
	a.mu.Lock();a.keys=next;a.fetched=time.Now();a.mu.Unlock();return nil
}
func str(v any)string{s,_:=v.(string);return s};func number(v any)float64{n,_:=v.(float64);return n};func audienceOK(v any,want string)bool{if s,ok:=v.(string);ok{return s==want};if xs,ok:=v.([]any);ok{for _,x:=range xs{if x==want{return true}}};return false}

// Hub fans list changes out to every connected client over SSE. Subscribers are
// push-only: a slow reader is skipped rather than allowed to block a mutation.
type sub struct{userID string;ch chan []byte}
type Hub struct{sync.Mutex;subs map[*sub]struct{}}
func newHub()*Hub{return &Hub{subs:map[*sub]struct{}{}}}
func (h *Hub) add(userID string)*sub{s:=&sub{userID:userID,ch:make(chan []byte,64)};h.Lock();h.subs[s]=struct{}{};h.Unlock();return s}
func (h *Hub) remove(s *sub){h.Lock();if _,ok:=h.subs[s];ok{delete(h.subs,s);close(s.ch)};h.Unlock()}
func (h *Hub) publish(to map[string]bool,f []byte){if f==nil{return};h.Lock();defer h.Unlock();for s:=range h.subs{if to[s.userID]{select{case s.ch<-f:default:}}}}
func frame(event string,payload any)[]byte{b,e:=json.Marshal(payload);if e!=nil{return nil};return []byte("event: "+event+"\ndata: "+string(b)+"\n\n")}
// recipients mirrors the visibility rule used by GET /v1/lists so a client is only
// told about a list it is also allowed to fetch.
func recipients(l List)map[string]bool{m:=map[string]bool{};if l.OwnerID!=""{m[l.OwnerID]=true};for uid,p:=range l.Permissions{if p!=""{m[uid]=true}};return m}
// origin identifies the device that caused a change so it can ignore its own echo.
func origin(r *http.Request,u User)string{if v:=strings.TrimSpace(r.Header.Get("X-Client-Id"));v!=""&&len(v)<=128{return v};return u.ID}

type API struct{s *Store;a *Auth;hub *Hub;pairingKey string}

// Constants of the device assertion. Kept narrow on purpose: an assertion is only ever
// meant for this connector, and only ever for a few minutes.
const deviceIssuer = "tidyshop-device"
const deviceAudience = "tidyshop-connector"
const deviceAssertionWindow = 300
const deviceClockSkew = 120

// seenIDs remembers assertion ids for as long as an assertion could still be valid, which is
// all that is needed to stop one being replayed. It is memory-only by design: a restart
// invalidates nothing that was not about to expire anyway.
type seenIDs struct{sync.Mutex;at map[string]int64}
func newSeenIDs()*seenIDs{return &seenIDs{at:map[string]int64{}}}
// One per process, at package level so it can never be nil on a partially built API.
var seenAssertions = newSeenIDs()
func (s *seenIDs) use(id string,now int64)bool{
	s.Lock();defer s.Unlock()
	for k,expiry:=range s.at{if expiry<now{delete(s.at,k)}}
	if _,replay:=s.at[id];replay{return false}
	s.at[id]=now+deviceAssertionWindow+deviceClockSkew
	return true
}

func (d Device) publicKey()(*ecdsa.PublicKey,error){
	xb,e:=base64.RawURLEncoding.DecodeString(d.X);if e!=nil{return nil,e}
	yb,e:=base64.RawURLEncoding.DecodeString(d.Y);if e!=nil{return nil,e}
	if len(xb)==0||len(yb)==0{return nil,errors.New("device key is incomplete")}
	return &ecdsa.PublicKey{Curve:elliptic.P256(),X:new(big.Int).SetBytes(xb),Y:new(big.Int).SetBytes(yb)},nil
}

func bearer(r *http.Request)string{return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"),"Bearer "))}

// tokenIssuer peeks at the issuer without verifying anything, purely to decide which
// verification path a token belongs to. Nothing is trusted on the strength of this.
func tokenIssuer(raw string)string{
	parts:=strings.Split(raw,".");if len(parts)!=3{return ""}
	b,e:=base64.RawURLEncoding.DecodeString(parts[1]);if e!=nil{return ""}
	var claims map[string]any;if json.Unmarshal(b,&claims)!=nil{return ""}
	return str(claims["iss"])
}

// identify resolves the caller from either a device assertion or a provider token.
func (api *API) identify(r *http.Request)(User,error){
	raw:=bearer(r)
	if raw==""{return User{},errors.New("missing bearer token")}
	if tokenIssuer(raw)==deviceIssuer{return api.deviceUser(raw)}
	if api.a==nil{return User{},errors.New("this server uses device enrollment; enroll this device with an invite or device-link code before continuing")}
	c,e:=api.a.claims(r);if e!=nil{return User{},e}
	u:=User{ID:c.Sub,Email:c.Email,FirstName:c.GivenName,LastName:c.FamilyName}
	if u.FirstName==""{parts:=strings.Fields(c.Name);if len(parts)>0{u.FirstName=parts[0]}else{u.FirstName=strings.Split(c.Email,"@")[0]}}
	return u,nil
}

// deviceUser verifies a device assertion and returns the identity it speaks for.
func (api *API) deviceUser(raw string)(User,error){
	parts:=strings.Split(raw,".");if len(parts)!=3{return User{},errors.New("invalid token")}
	hb,e:=base64.RawURLEncoding.DecodeString(parts[0]);if e!=nil{return User{},e}
	var head map[string]any;if e=json.Unmarshal(hb,&head);e!=nil{return User{},e}
	if str(head["alg"])!="ES256"{return User{},errors.New("device assertion must use ES256")}
	kid:=str(head["kid"]);if kid==""{return User{},errors.New("device assertion has no key id")}
	api.s.RLock();device,known:=api.s.Devices[kid];user,hasUser:=api.s.KnownUsers[device.UserID];api.s.RUnlock()
	if !known{return User{},errors.New("this device is not enrolled")}
	key,e:=device.publicKey();if e!=nil{return User{},e}
	sig,e:=base64.RawURLEncoding.DecodeString(parts[2]);if e!=nil{return User{},e}
	sum:=sha256.Sum256([]byte(parts[0]+"."+parts[1]))
	if e=verifySignature(key,"ES256",sum[:],sig);e!=nil{return User{},e}
	pb,e:=base64.RawURLEncoding.DecodeString(parts[1]);if e!=nil{return User{},e}
	var claims map[string]any;if e=json.Unmarshal(pb,&claims);e!=nil{return User{},e}
	now:=time.Now().Unix()
	exp:=int64(number(claims["exp"]));iat:=int64(number(claims["iat"]))
	switch{
	case str(claims["aud"])!=deviceAudience: return User{},errors.New("device assertion is for another service")
	case str(claims["sub"])!=device.UserID: return User{},errors.New("device assertion does not match its device")
	case exp<=now: return User{},errors.New("device assertion has expired")
	case exp>now+deviceAssertionWindow+deviceClockSkew: return User{},errors.New("device assertion lives too long")
	case iat>now+deviceClockSkew: return User{},errors.New("device assertion is from the future")
	}
	jti:=str(claims["jti"]);if len(jti)<8{return User{},errors.New("device assertion has no usable id")}
	if !seenAssertions.use(jti,now){return User{},errors.New("device assertion has already been used")}
	api.touchDevice(kid,now*1000)
	if !hasUser{return User{},errors.New("this device belongs to an identity that no longer exists")}
	return user,nil
}

// touchDevice records activity, but writes at most once a minute so an active device does
// not cause a disk write on every request.
func (api *API) touchDevice(id string,atMillis int64){
	api.s.Lock();defer api.s.Unlock()
	d,ok:=api.s.Devices[id];if !ok{return}
	if atMillis-d.LastSeenAt<60_000{return}
	d.LastSeenAt=atMillis;api.s.Devices[id]=d;_ = api.s.save()
}
func (api *API) emit(event string,to map[string]bool,payload map[string]any){api.hub.publish(to,frame(event,payload))}
// events streams list changes to one client until it disconnects. The per-request
// write deadline is cleared so the server-wide WriteTimeout cannot cut the stream.
func (api *API) events(w http.ResponseWriter,r *http.Request,u User){
	rc:=http.NewResponseController(w);_=rc.SetWriteDeadline(time.Time{});_=rc.SetReadDeadline(time.Time{})
	h:=w.Header();h.Set("Content-Type","text/event-stream");h.Set("Cache-Control","no-cache, no-transform");h.Set("Connection","keep-alive");h.Set("X-Accel-Buffering","no")
	w.WriteHeader(200)
	s:=api.hub.add(u.ID);defer api.hub.remove(s)
	api.s.RLock();out:=[]List{};for _,l:=range api.s.Lists{if permission(l,u.ID)!=""{out=append(out,publicList(l))}};api.s.RUnlock()
	// A full snapshot on connect makes a reconnecting client consistent without polling.
	if _,e:=w.Write(frame("sync",map[string]any{"lists":out}));e!=nil{return}
	if e:=rc.Flush();e!=nil{return}
	ping:=time.NewTicker(25*time.Second);defer ping.Stop()
	for{
		select{
		case <-r.Context().Done(): return
		case f,ok:=<-s.ch: if !ok{return};if _,e:=w.Write(f);e!=nil{return};if e:=rc.Flush();e!=nil{return}
		case <-ping.C: if _,e:=w.Write([]byte(": ping\n\n"));e!=nil{return};if e:=rc.Flush();e!=nil{return}
		}
	}
}
func (api *API) deleteList(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimPrefix(r.URL.Path,"/v1/lists/")
	api.s.Lock();l,ok:=api.s.Lists[idv]
	if !ok{api.s.Unlock();write(w,404,map[string]string{"error":"list not found"});return}
	if l.OwnerID!=u.ID{api.s.Unlock();write(w,403,map[string]string{"error":"only the list owner can delete this list"});return}
	delete(api.s.Lists,idv);api.s.DeletedLists[idv]=time.Now().UnixMilli();_=api.s.save();api.s.Unlock()
	api.emit("list.deleted",recipients(l),map[string]any{"origin":origin(r,u),"listId":idv})
	write(w,200,map[string]any{"deleted":idv})
}

// adminLists exposes only management metadata. Item contents and suggestion history
// deliberately never leave the connector through this endpoint.
func (api *API) adminLists(w http.ResponseWriter,u User){
	api.s.RLock();defer api.s.RUnlock()
	var h Household;found:=false
	for _,candidate:=range api.s.Households{if member(candidate,u.ID)&&isOwner(candidate,u.ID){h=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"only a family administrator can manage all shared lists"});return}
	out:=[]map[string]any{}
	for _,l:=range api.s.Lists{
		if l.HouseholdID!=h.ID{continue}
		ownerName:="Former member";ownerPresent:=false
		for _,m:=range h.Members{if m.User.ID==l.OwnerID{ownerName=strings.TrimSpace(m.User.FirstName+" "+m.User.LastName);if ownerName==""{ownerName=m.User.Email};if ownerName==""{ownerName="Family member"};ownerPresent=true;break}}
		countryCode:=strings.ToUpper(strings.TrimSpace(l.StoreCountryCode));countryName:=strings.TrimSpace(l.StoreCountryName);if countryCode==""&&strings.HasPrefix(l.Icon,"store:"){parts:=strings.SplitN(l.Icon,":",3);if len(parts)==3{countryCode=strings.ToUpper(parts[1])}};if countryName==""{switch countryCode{case "NL":countryName="Netherlands";case "DE":countryName="Germany";case "BE":countryName="Belgium";case "FR":countryName="France"}}
		out=append(out,map[string]any{"id":l.ID,"title":l.Title,"icon":l.Icon,"countryCode":countryCode,"countryName":countryName,"ownerName":ownerName,"ownerPresent":ownerPresent,"accessCount":len(l.Permissions),"updatedAt":l.UpdatedAt})
	}
	write(w,200,out)
}
func (api *API) deleteAdminList(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimPrefix(r.URL.Path,"/v1/admin/lists/")
	api.s.Lock();var h Household;found:=false
	for _,candidate:=range api.s.Households{if member(candidate,u.ID)&&isOwner(candidate,u.ID){h=candidate;found=true;break}}
	if !found{api.s.Unlock();write(w,403,map[string]string{"error":"only a family administrator can remove shared lists"});return}
	l,ok:=api.s.Lists[idv];if !ok||l.HouseholdID!=h.ID{api.s.Unlock();write(w,404,map[string]string{"error":"shared list not found"});return}
	to:=recipients(l);for _,m:=range h.Members{to[m.User.ID]=true}
	delete(api.s.Lists,idv);api.s.DeletedLists[idv]=time.Now().UnixMilli();_=api.s.save();api.s.Unlock()
	api.emit("list.deleted",to,map[string]any{"origin":"connector-admin","listId":idv})
	write(w,200,map[string]any{"deleted":idv})
}
func (api *API) ServeHTTP(w http.ResponseWriter,r *http.Request){
	if r.URL.Path=="/icon.png"&&r.Method=="GET"{http.ServeFile(w,r,env("ICON_FILE","/data/icon.png"));return}
	w.Header().Set("Content-Type","application/json");if r.URL.Path=="/health"{write(w,200,map[string]any{"status":"ok","service":"TidyShop-Connector"});return}
	if r.URL.Path=="/v1/devices/enrol"&&r.Method=="POST"{api.enrolDevice(w,r);return}
	if r.URL.Path=="/v1/devices/claimable"&&r.Method=="POST"{api.claimableIdentities(w,r);return}
	u,e:=api.identify(r);if e!=nil{write(w,401,map[string]string{"error":e.Error()});return}
	switch{
	case r.URL.Path=="/v1/me"&&r.Method=="GET": write(w,200,u)
	case r.URL.Path=="/v1/pair"&&r.Method=="POST": api.pair(w,r,u)
	case !api.known(u.ID): write(w,403,map[string]string{"error":"user is not enrolled on this family server"})
	case r.URL.Path=="/v1/events"&&r.Method=="GET": api.events(w,r,u)
	case r.URL.Path=="/v1/devices"&&r.Method=="GET": api.listDevices(w,u)
	case r.URL.Path=="/v1/devices/link"&&r.Method=="POST": api.linkDevice(w,r,u)
	case r.URL.Path=="/v1/devices/recovery"&&r.Method=="POST": api.rotateRecovery(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/devices/")&&r.Method=="DELETE": api.revokeDevice(w,r,u)
	case r.URL.Path=="/v1/backups"&&r.Method=="POST": api.createBackup(w,r,u)
	case r.URL.Path=="/v1/backups"&&r.Method=="GET": api.listBackups(w,u)
	case strings.HasPrefix(r.URL.Path,"/v1/backups/")&&r.Method=="GET": api.fetchBackup(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/backups/")&&r.Method=="DELETE": api.deleteBackup(w,r,u)
	case r.URL.Path=="/v1/invites"&&r.Method=="POST": api.createInvite(w,r,u)
	case r.URL.Path=="/v1/invites"&&r.Method=="GET": api.listInvites(w,u)
	case strings.HasPrefix(r.URL.Path,"/v1/invites/")&&r.Method=="DELETE": api.revokeInvite(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/members/")&&strings.HasSuffix(r.URL.Path,"/invite-permission")&&r.Method=="PUT": api.updateInvitePermission(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/members/")&&strings.HasSuffix(r.URL.Path,"/device-permission")&&r.Method=="PUT": api.updateDevicePermission(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/members/")&&r.Method=="DELETE": api.removeMember(w,r,u)
	case r.URL.Path=="/v1/households/admin"&&r.Method=="PUT": api.claimHouseholdAdmin(w,r,u)
	case r.URL.Path=="/v1/members"&&r.Method=="GET": api.members(w)
	case r.URL.Path=="/v1/households"&&r.Method=="GET": api.households(w,u)
	case r.URL.Path=="/v1/households"&&r.Method=="POST": api.createHousehold(w,r,u)
	case r.URL.Path=="/v1/admin/lists"&&r.Method=="GET": api.adminLists(w,u)
	case strings.HasPrefix(r.URL.Path,"/v1/admin/lists/")&&r.Method=="DELETE": api.deleteAdminList(w,r,u)
	case r.URL.Path=="/v1/lists/subscribable"&&r.Method=="GET": api.subscribableLists(w,u)
	case r.URL.Path=="/v1/lists"&&r.Method=="GET": api.lists(w,u)
	case r.URL.Path=="/v1/lists"&&r.Method=="POST": api.createList(w,r,u)
	case strings.HasSuffix(r.URL.Path,"/subscribe")&&r.Method=="POST": api.subscribeToList(w,r,u)
	case strings.HasSuffix(r.URL.Path,"/nudge")&&r.Method=="POST": api.nudgeList(w,r,u)
	case strings.HasSuffix(r.URL.Path,"/permissions")&&r.Method=="PUT": api.updatePermissions(w,r,u)
	case strings.HasSuffix(r.URL.Path,"/suggestions")&&r.Method=="POST": api.recordSuggestion(w,r,u)
	case strings.Contains(r.URL.Path,"/suggestions/")&&r.Method=="DELETE": api.removeSuggestion(w,r,u)
	case strings.HasSuffix(r.URL.Path,"/suggestions")&&r.Method=="DELETE": api.resetSuggestions(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/lists/")&&r.Method=="PUT": api.updateList(w,r,u)
	case strings.HasPrefix(r.URL.Path,"/v1/lists/")&&r.Method=="DELETE": api.deleteList(w,r,u)
	default: write(w,404,map[string]string{"error":"not found"})}
}
func (api *API) known(id string)bool{api.s.RLock();defer api.s.RUnlock();_,ok:=api.s.KnownUsers[id];return ok}
func (api *API) pair(w http.ResponseWriter,r *http.Request,u User){
	var in struct{PairingKey,FamilyName,InviteCode string};if decode(w,r,&in)!=nil{return}
	api.s.Lock();defer api.s.Unlock()
	now:=time.Now().UnixMilli()
	var viaInvite *Invite
	if code:=normalizeCode(in.InviteCode);code!=""{
		inv,ok:=api.s.Invites[code]
		if !ok||inv.UsedAt!=0||now>=inv.ExpiresAt{write(w,403,map[string]string{"error":"that invite is not valid any more"});return}
		if _,exists:=api.s.Households[inv.HouseholdID];!exists{write(w,403,map[string]string{"error":"that invite points at a household that no longer exists"});return}
		viaInvite=&inv
	}else if len(api.pairingKey)<12||subtle.ConstantTimeCompare([]byte(in.PairingKey),[]byte(api.pairingKey))!=1{
		write(w,403,map[string]string{"error":"pairing key rejected"});return
	}
	api.s.KnownUsers[u.ID]=u
	var h Household;var found bool
	if viaInvite!=nil{
		// An invite names the household to join, so it never creates a new one.
		h=api.s.Households[viaInvite.HouseholdID];found=true
	}else{
		for _,candidate:=range api.s.Households{h=candidate;found=true;break}
	}
	if !found{
		name:=strings.TrimSpace(in.FamilyName);if name==""{name="Family"}
		h=Household{ID:id(),Name:name,Members:[]Member{{User:u,Role:"owner",CanInvite:true,AllowMultipleDevices:true,JoinedAt:now}},CreatedAt:now,UpdatedAt:now}
	}else if !member(h,u.ID){
		h.Members=append(h.Members,Member{User:u,Role:"member",JoinedAt:now});h.UpdatedAt=now
	}
	api.s.Households[h.ID]=h
	if viaInvite!=nil{viaInvite.UsedAt=now;viaInvite.UsedBy=u.ID;api.s.Invites[viaInvite.Code]=*viaInvite}
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	api.emit("household.updated",householdRecipients(h),map[string]any{"origin":origin(r,u),"household":householdView(h)})
	write(w,200,householdView(h))
}
// Codes are shown and typed by people, so matching ignores case and separators.
func normalizeCode(v string)string{
	var b strings.Builder
	for _,r:=range strings.ToUpper(strings.TrimSpace(v)){
		if (r>='A'&&r<='Z')||(r>='0'&&r<='9'){b.WriteRune(r)}
	}
	return b.String()
}
// The alphabet omits characters that are easy to confuse when read aloud or retyped.
const inviteAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
func newInviteCode()(string,error){
	b:=make([]byte,8)
	if _,e:=cryptorand.Read(b);e!=nil{return "",e}
	out:=make([]byte,8)
	for i,v:=range b{out[i]=inviteAlphabet[int(v)%len(inviteAlphabet)]}
	return string(out),nil
}
// Recovery codes are 20 characters of the invite alphabet -- a little under 100 bits -- so a
// single SHA-256 is enough here. There is nothing worth grinding through, and unlike a
// password there is no low-entropy guess to make.
func newRecoveryCode()(string,error){
	b:=make([]byte,20)
	if _,e:=cryptorand.Read(b);e!=nil{return "",e}
	out:=make([]byte,0,23)
	for i,v:=range b{
		if i>0&&i%5==0{out=append(out,'-')}
		out=append(out,inviteAlphabet[int(v)%len(inviteAlphabet)])
	}
	return string(out),nil
}
func recoveryHash(code string)string{sum:=sha256.Sum256([]byte(normalizeCode(code)));return fmt.Sprintf("%x",sum)}
// mintRecovery replaces whatever code the user held before, so a code that was written down
// and then lost stops working. The caller holds the store lock.
func (api *API) mintRecovery(userID string)(string,error){
	code,e:=newRecoveryCode();if e!=nil{return "",e}
	if api.s.RecoveryCodes==nil{api.s.RecoveryCodes=map[string]string{}}
	api.s.RecoveryCodes[userID]=recoveryHash(code)
	return code,nil
}
// userForRecovery compares in constant time and without stopping early, so neither a wrong
// code nor its position in the table is revealed by how long the answer takes.
func (api *API) userForRecovery(code string)(string,bool){
	if normalizeCode(code)==""{return "",false}
	want:=recoveryHash(code);match:=""
	for userID,held:=range api.s.RecoveryCodes{
		if subtle.ConstantTimeCompare([]byte(held),[]byte(want))==1{match=userID}
	}
	return match,match!=""
}
// householdFor finds the household an identity already belongs to. Re-linking never picks a
// household: it goes where the identity already is. The caller holds the store lock.
func (api *API) householdFor(userID string)(Household,bool){
	for _,candidate:=range api.s.Households{if member(candidate,userID){return candidate,true}}
	return Household{},false
}
func inviteView(i Invite)map[string]any{return map[string]any{"code":i.Code,"householdId":i.HouseholdID,"createdAt":i.CreatedAt,"expiresAt":i.ExpiresAt,"usedAt":i.UsedAt,"usedBy":i.UsedBy,"subject":i.Subject,"kind":map[bool]string{true:"device",false:"member"}[i.Subject!=""]}}
// Keeping a bounded history means the state file cannot grow without limit while still
// letting someone roll back a few versions.
const maxBackupsPerUser = 14
const maxBackupBytes = 2 << 20
func backupView(b Backup)map[string]any{return map[string]any{"id":b.ID,"name":b.Name,"createdAt":b.CreatedAt,"size":b.Size}}
func (api *API) createBackup(w http.ResponseWriter,r *http.Request,u User){
	var in struct{Name,Payload string}
	if decodeLimit(w,r,&in,maxBackupBytes+(1<<16))!=nil{return}
	if strings.TrimSpace(in.Payload)==""{write(w,400,map[string]string{"error":"payload is required"});return}
	if len(in.Payload)>maxBackupBytes{write(w,413,map[string]string{"error":"backup is too large"});return}
	name:=strings.TrimSpace(in.Name);if name==""||len(name)>160{name="TidyShop-backup.json"}
	api.s.Lock();defer api.s.Unlock()
	b:=Backup{ID:id(),Name:name,Payload:in.Payload,CreatedAt:time.Now().UnixMilli(),Size:len(in.Payload)}
	kept:=append([]Backup{b},api.s.Backups[u.ID]...)
	if len(kept)>maxBackupsPerUser{kept=kept[:maxBackupsPerUser]}
	api.s.Backups[u.ID]=kept
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,201,backupView(b))
}
func (api *API) listBackups(w http.ResponseWriter,u User){
	api.s.RLock();defer api.s.RUnlock()
	out:=[]map[string]any{}
	for _,b:=range api.s.Backups[u.ID]{out=append(out,backupView(b))}
	write(w,200,out)
}
func (api *API) fetchBackup(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimPrefix(r.URL.Path,"/v1/backups/")
	api.s.RLock();defer api.s.RUnlock()
	for _,b:=range api.s.Backups[u.ID]{
		if b.ID==idv{write(w,200,map[string]any{"id":b.ID,"name":b.Name,"createdAt":b.CreatedAt,"payload":b.Payload});return}
	}
	write(w,404,map[string]string{"error":"backup not found"})
}
func (api *API) deleteBackup(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimPrefix(r.URL.Path,"/v1/backups/")
	api.s.Lock();defer api.s.Unlock()
	kept:=[]Backup{};found:=false
	for _,b:=range api.s.Backups[u.ID]{if b.ID==idv{found=true;continue};kept=append(kept,b)}
	if !found{write(w,404,map[string]string{"error":"backup not found"});return}
	api.s.Backups[u.ID]=kept
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,200,map[string]any{"deleted":idv})
}
func deviceView(d Device)map[string]any{return map[string]any{"id":d.ID,"userId":d.UserID,"name":d.Name,"enrolledAt":d.EnrolledAt,"lastSeenAt":d.LastSeenAt}}

// enrolDevice is the only unauthenticated write on the server. It is gated by a single-use
// invite code, a recovery code, or the master pairing key, and it is what mints a credential
// for a device that does not have one yet.
//
// Three of its paths admit a device to an identity that already exists, which is what a
// reinstalled phone needs: uninstalling destroys the keystore key, and without one of these
// the phone could only ever enrol as a stranger, owning none of its own lists or backups.
func (api *API) enrolDevice(w http.ResponseWriter,r *http.Request){
	var in struct{
		Code,PairingKey,DeviceName,DisplayName,Email,ClaimUserID,RecoveryCode string
		PublicKey struct{Kty,Crv,X,Y string}
	}
	if decode(w,r,&in)!=nil{return}
	if in.PublicKey.Kty!="EC"||in.PublicKey.Crv!="P-256"{write(w,400,map[string]string{"error":"an EC P-256 public key is required"});return}
	if in.PublicKey.X==""||in.PublicKey.Y==""{write(w,400,map[string]string{"error":"the public key is incomplete"});return}
	api.s.Lock();defer api.s.Unlock()
	now:=time.Now().UnixMilli()
	var household Household;var subject string;var viaInvite *Invite;viaMasterKey:=false;viaRecovery:=false
	switch{
	case normalizeCode(in.RecoveryCode)!="":
		// A recovery code names its own identity, so it needs neither an invite nor the
		// pairing key. This is the path for someone who does not administer the server.
		owner,ok:=api.userForRecovery(in.RecoveryCode)
		if !ok{write(w,403,map[string]string{"error":"that recovery code is not valid"});return}
		h,found:=api.householdFor(owner)
		if !found{write(w,403,map[string]string{"error":"that identity is no longer in a household"});return}
		household=h;subject=owner;viaRecovery=true
	case normalizeCode(in.Code)!="":
		if strings.TrimSpace(in.ClaimUserID)!=""{write(w,400,map[string]string{"error":"an invite code already names who it admits"});return}
		inv,ok:=api.s.Invites[normalizeCode(in.Code)]
		if !ok||inv.UsedAt!=0||now>=inv.ExpiresAt{write(w,403,map[string]string{"error":"that code is not valid any more"});return}
		h,exists:=api.s.Households[inv.HouseholdID]
		if !exists{write(w,403,map[string]string{"error":"that code points at a household that no longer exists"});return}
		household=h;subject=inv.Subject;viaInvite=&inv
	case len(api.pairingKey)>=12&&subtle.ConstantTimeCompare([]byte(in.PairingKey),[]byte(api.pairingKey))==1:
		viaMasterKey=true
		if claim:=strings.TrimSpace(in.ClaimUserID);claim!=""{
			// Re-linking a reinstalled phone to the identity it had before. The pairing key
			// already admits its holder to the household as an owner, so this widens what that
			// key reaches rather than who may use it.
			if _,ok:=api.s.KnownUsers[claim];!ok{write(w,403,map[string]string{"error":"that identity no longer exists"});return}
			h,found:=api.householdFor(claim)
			if !found{write(w,403,map[string]string{"error":"that identity is no longer in a household"});return}
			household=h;subject=claim
			break
		}
		for _,candidate:=range api.s.Households{household=candidate;break}
		if household.ID==""{
			name:=strings.TrimSpace(in.DisplayName);if name==""{name="Family"}
			household=Household{ID:id(),Name:name+"'s family",Members:[]Member{},CreatedAt:now,UpdatedAt:now}
		}
	default:
		write(w,403,map[string]string{"error":"an invite code, a recovery code, or the pairing key is required"});return
	}
	userID:=subject
	if userID!=""{
		// Linking another device to an identity that must already exist.
		if _,ok:=api.s.KnownUsers[userID];!ok{write(w,403,map[string]string{"error":"that code points at an identity that no longer exists"});return}
	}else{
		name:=strings.TrimSpace(in.DisplayName);if name==""{name="Member"}
		userID=id()
		api.s.KnownUsers[userID]=User{ID:userID,Email:strings.TrimSpace(in.Email),FirstName:name}
	}
	// A member held to one device is replacing it here, not adding to it. Refusing is what
	// stranded a reinstalled phone: the install that held the old key is gone, and the cap is
	// about how many devices are live at once rather than how many may ever enrol.
	if subject!=""&&countDevices(api.s.Devices,userID)>0&&!mayUseMultipleDevices(household,userID){
		for deviceID,d:=range api.s.Devices{if d.UserID==userID{delete(api.s.Devices,deviceID)}}
	}
	if !member(household,userID){
		role:="member";if len(household.Members)==0||(viaMasterKey&&subject==""){role="owner"}
		household.Members=append(household.Members,Member{User:api.s.KnownUsers[userID],Role:role,CanInvite:role=="owner",AllowMultipleDevices:role=="owner",JoinedAt:now})
		household.UpdatedAt=now
	}
	api.s.Households[household.ID]=household
	name:=strings.TrimSpace(in.DeviceName);if name==""||len(name)>80{name="A device"}
	device:=Device{ID:id(),UserID:userID,Name:name,X:in.PublicKey.X,Y:in.PublicKey.Y,EnrolledAt:now,LastSeenAt:now}
	api.s.Devices[device.ID]=device
	if viaInvite!=nil{viaInvite.UsedAt=now;viaInvite.UsedBy=userID;api.s.Invites[viaInvite.Code]=*viaInvite}
	// A recovery code is single-use, so its replacement goes back with the response: a phone
	// that just used one is never left without a way back in. Identities enrolled before
	// recovery codes existed are given their first one here.
	recovery:=""
	if _,held:=api.s.RecoveryCodes[userID];viaRecovery||!held{
		code,e:=api.mintRecovery(userID)
		if e!=nil{write(w,500,map[string]string{"error":"could not generate a recovery code"});return}
		recovery=code
	}
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	api.emit("household.updated",householdRecipients(household),map[string]any{"origin":origin(r,User{ID:userID}),"household":householdView(household)})
	out:=map[string]any{"deviceId":device.ID,"userId":userID,"household":householdView(household)}
	if recovery!=""{out["recoveryCode"]=recovery}
	write(w,201,out)
}

// claimableIdentities lists who a reinstalled phone could re-link as. It is gated by the
// master pairing key, which already admits its holder to the household as an owner, so it
// discloses nothing that key could not already reach.
func (api *API) claimableIdentities(w http.ResponseWriter,r *http.Request){
	var in struct{PairingKey string}
	if decode(w,r,&in)!=nil{return}
	if len(api.pairingKey)<12||subtle.ConstantTimeCompare([]byte(in.PairingKey),[]byte(api.pairingKey))!=1{write(w,403,map[string]string{"error":"pairing key rejected"});return}
	api.s.RLock();defer api.s.RUnlock()
	out:=[]map[string]any{}
	for _,h:=range api.s.Households{
		for _,m:=range h.Members{
			out=append(out,map[string]any{"userId":m.User.ID,"displayName":strings.TrimSpace(m.User.FirstName+" "+m.User.LastName),
				"householdId":h.ID,"householdName":h.Name,"role":m.Role,"devices":countDevices(api.s.Devices,m.User.ID),"joinedAt":m.JoinedAt})
		}
	}
	write(w,200,out)
}

// rotateRecovery hands the caller a fresh recovery code and forgets the old one. The code is
// shown once and never stored on the phone, so this is also how someone who did not write the
// last one down ends up with a code they actually have.
func (api *API) rotateRecovery(w http.ResponseWriter,r *http.Request,u User){
	api.s.Lock();defer api.s.Unlock()
	code,e:=api.mintRecovery(u.ID)
	if e!=nil{write(w,500,map[string]string{"error":"could not generate a recovery code"});return}
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,201,map[string]any{"recoveryCode":code})
}

func (api *API) listDevices(w http.ResponseWriter,u User){
	api.s.RLock();defer api.s.RUnlock()
	out:=[]map[string]any{}
	for _,d:=range api.s.Devices{if d.UserID==u.ID{out=append(out,deviceView(d))}}
	write(w,200,out)
}

// linkDevice mints a code for another device belonging to the caller, rather than for a new
// member, so a second phone joins the same identity. An owner may name another member
// instead, which is how a household re-admits somebody else's reinstalled phone.
func (api *API) linkDevice(w http.ResponseWriter,r *http.Request,u User){
	var in struct{ExpiresInHours int;MemberID string}
	if decode(w,r,&in)!=nil{return}
	hours:=in.ExpiresInHours;if hours<=0{hours=1};if hours>24{hours=24}
	api.s.Lock();defer api.s.Unlock()
	var household Household;var found bool
	for _,candidate:=range api.s.Households{if member(candidate,u.ID){household=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"join a household before linking a device"});return}
	target:=u.ID
	if claim:=strings.TrimSpace(in.MemberID);claim!=""&&claim!=u.ID{
		if !isOwner(household,u.ID){write(w,403,map[string]string{"error":"only a family administrator can link another member's device"});return}
		if !member(household,claim){write(w,404,map[string]string{"error":"that member is not in this household"});return}
		target=claim
	}
	// The one-device cap is the caller's own limit. An owner minting for someone else is not
	// adding a device to that member either: enrolling replaces the one they had.
	if target==u.ID&&countDevices(api.s.Devices,u.ID)>0&&!mayUseMultipleDevices(household,u.ID){write(w,403,map[string]string{"error":"a family administrator has limited this member to one device"});return}
	code,e:=newInviteCode();if e!=nil{write(w,500,map[string]string{"error":"could not generate a code"});return}
	now:=time.Now().UnixMilli()
	inv:=Invite{Code:code,HouseholdID:household.ID,CreatedBy:u.ID,Subject:target,CreatedAt:now,ExpiresAt:now+int64(hours)*3600*1000}
	api.s.Invites[code]=inv
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,201,inviteView(inv))
}

func (api *API) revokeDevice(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimPrefix(r.URL.Path,"/v1/devices/")
	api.s.Lock();defer api.s.Unlock()
	d,ok:=api.s.Devices[idv]
	if !ok{write(w,404,map[string]string{"error":"device not found"});return}
	if d.UserID!=u.ID{write(w,403,map[string]string{"error":"that device belongs to someone else"});return}
	delete(api.s.Devices,idv)
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,200,map[string]any{"revoked":idv})
}

func (api *API) createInvite(w http.ResponseWriter,r *http.Request,u User){
	var in struct{ExpiresInHours int}
	if decode(w,r,&in)!=nil{return}
	hours:=in.ExpiresInHours;if hours<=0{hours=24};if hours>168{hours=168}
	api.s.Lock();defer api.s.Unlock()
	var h Household;var found bool
	for _,candidate:=range api.s.Households{if member(candidate,u.ID){h=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"join a household before inviting anyone"});return}
	if !mayInvite(h,u.ID){write(w,403,map[string]string{"error":"a family admin has not allowed this member to invite new people"});return}
	now:=time.Now().UnixMilli()
	var code string
	for attempt:=0;attempt<8;attempt++{
		c,e:=newInviteCode();if e!=nil{write(w,500,map[string]string{"error":"could not generate a code"});return}
		if _,clash:=api.s.Invites[c];!clash{code=c;break}
	}
	if code==""{write(w,500,map[string]string{"error":"could not generate a code"});return}
	// Drop invites that are spent or long expired so the file cannot grow without bound.
	for c,old:=range api.s.Invites{if old.UsedAt!=0||now>old.ExpiresAt+7*24*3600*1000{delete(api.s.Invites,c)}}
	inv:=Invite{Code:code,HouseholdID:h.ID,CreatedBy:u.ID,CreatedAt:now,ExpiresAt:now+int64(hours)*3600*1000}
	api.s.Invites[code]=inv
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,201,inviteView(inv))
}
func (api *API) listInvites(w http.ResponseWriter,u User){
	api.s.RLock();defer api.s.RUnlock()
	now:=time.Now().UnixMilli();out:=[]map[string]any{}
	for _,inv:=range api.s.Invites{
		if inv.UsedAt!=0||now>=inv.ExpiresAt{continue}
		if h,ok:=api.s.Households[inv.HouseholdID];ok&&member(h,u.ID)&&(inv.CreatedBy==u.ID||isOwner(h,u.ID)){out=append(out,inviteView(inv))}
	}
	write(w,200,out)
}
func (api *API) revokeInvite(w http.ResponseWriter,r *http.Request,u User){
	code:=normalizeCode(strings.TrimPrefix(r.URL.Path,"/v1/invites/"))
	api.s.Lock();defer api.s.Unlock()
	inv,ok:=api.s.Invites[code]
	if !ok{write(w,404,map[string]string{"error":"invite not found"});return}
	h,exists:=api.s.Households[inv.HouseholdID]
	if !exists||!member(h,u.ID){write(w,403,map[string]string{"error":"that invite belongs to another household"});return}
	if inv.CreatedBy!=u.ID&&!isOwner(h,u.ID){write(w,403,map[string]string{"error":"you cannot revoke another member's invite"});return}
	delete(api.s.Invites,code)
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	write(w,200,map[string]any{"revoked":code})
}
func householdRecipients(h Household)map[string]bool{out:=map[string]bool{};for _,m:=range h.Members{out[m.User.ID]=true};return out}
func mayInvite(h Household,userID string)bool{for _,m:=range h.Members{if m.User.ID==userID{return m.Role=="owner"||m.CanInvite}};return false}
func mayUseMultipleDevices(h Household,userID string)bool{for _,m:=range h.Members{if m.User.ID==userID{return m.Role=="owner"||m.AllowMultipleDevices}};return false}
func countDevices(devices map[string]Device,userID string)int{count:=0;for _,d:=range devices{if d.UserID==userID{count++}};return count}
func isOwner(h Household,userID string)bool{for _,m:=range h.Members{if m.User.ID==userID{return m.Role=="owner"}};return false}
func (api *API) claimHouseholdAdmin(w http.ResponseWriter,r *http.Request,u User){
	var in struct{PairingKey string};if decode(w,r,&in)!=nil{return}
	if len(api.pairingKey)<12||subtle.ConstantTimeCompare([]byte(in.PairingKey),[]byte(api.pairingKey))!=1{write(w,403,map[string]string{"error":"the master pairing key is incorrect"});return}
	api.s.Lock();defer api.s.Unlock()
	var h Household;var found bool
	for _,candidate:=range api.s.Households{if member(candidate,u.ID){h=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"join a household before claiming administration"});return}
	for i,m:=range h.Members{if m.User.ID==u.ID{h.Members[i].Role="owner";h.Members[i].CanInvite=true;h.Members[i].AllowMultipleDevices=true;break}}
	h.UpdatedAt=time.Now().UnixMilli();api.s.Households[h.ID]=h
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	api.emit("household.updated",householdRecipients(h),map[string]any{"origin":origin(r,u),"household":householdView(h)})
	write(w,200,householdView(h))
}
func (api *API) updateInvitePermission(w http.ResponseWriter,r *http.Request,u User){
	userID:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/members/"),"/invite-permission")
	var in struct{Allow bool};if decode(w,r,&in)!=nil{return}
	api.s.Lock();defer api.s.Unlock()
	var h Household;var found bool
	for _,candidate:=range api.s.Households{if member(candidate,u.ID){h=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"join a household first"});return}
	if !isOwner(h,u.ID){write(w,403,map[string]string{"error":"only the family owner can change invitation access"});return}
	changed:=false
	for i,m:=range h.Members{
		if m.User.ID!=userID{continue}
		if m.Role=="owner"&&!in.Allow{write(w,400,map[string]string{"error":"the family owner always keeps invitation access"});return}
		h.Members[i].CanInvite=in.Allow;changed=true;break
	}
	if !changed{write(w,404,map[string]string{"error":"family member not found"});return}
	h.UpdatedAt=time.Now().UnixMilli();api.s.Households[h.ID]=h
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	api.emit("household.updated",householdRecipients(h),map[string]any{"origin":origin(r,u),"household":householdView(h)})
	write(w,200,householdView(h))
}
func (api *API) updateDevicePermission(w http.ResponseWriter,r *http.Request,u User){
	userID:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/members/"),"/device-permission")
	var in struct{Allow bool};if decode(w,r,&in)!=nil{return}
	api.s.Lock();defer api.s.Unlock()
	var h Household;var found bool
	for _,candidate:=range api.s.Households{if member(candidate,u.ID){h=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"join a household first"});return}
	if !isOwner(h,u.ID){write(w,403,map[string]string{"error":"only a family administrator can change device access"});return}
	changed:=false
	for i,m:=range h.Members{
		if m.User.ID!=userID{continue}
		if m.Role=="owner"&&!in.Allow{write(w,400,map[string]string{"error":"family administrators may link their own devices"});return}
		h.Members[i].AllowMultipleDevices=in.Allow;changed=true;break
	}
	if !changed{write(w,404,map[string]string{"error":"family member not found"});return}
	h.UpdatedAt=time.Now().UnixMilli();api.s.Households[h.ID]=h
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	api.emit("household.updated",householdRecipients(h),map[string]any{"origin":origin(r,u),"household":householdView(h)})
	write(w,200,householdView(h))
}
func (api *API) removeMember(w http.ResponseWriter,r *http.Request,u User){
	targetID:=strings.TrimSpace(strings.TrimPrefix(r.URL.Path,"/v1/members/"))
	if targetID==""{write(w,400,map[string]string{"error":"member id required"});return}
	api.s.Lock();defer api.s.Unlock()
	var h Household;var found bool
	for _,candidate:=range api.s.Households{if member(candidate,u.ID){h=candidate;found=true;break}}
	if !found{write(w,403,map[string]string{"error":"join a household first"});return}
	if !isOwner(h,u.ID){write(w,403,map[string]string{"error":"only a family administrator can remove members"});return}
	if targetID==u.ID{write(w,400,map[string]string{"error":"use Leave family instead of removing yourself"});return}
	oldRecipients:=householdRecipients(h);remaining:=make([]Member,0,len(h.Members)-1);removed:=false
	for _,m:=range h.Members{if m.User.ID==targetID{removed=true;continue};remaining=append(remaining,m)}
	if !removed{write(w,404,map[string]string{"error":"family member not found"});return}
	h.Members=remaining;h.UpdatedAt=time.Now().UnixMilli();api.s.Households[h.ID]=h
	for deviceID,d:=range api.s.Devices{if d.UserID==targetID{delete(api.s.Devices,deviceID)}}
	for code,inv:=range api.s.Invites{if inv.CreatedBy==targetID||inv.Subject==targetID{delete(api.s.Invites,code)}}
	delete(api.s.RecoveryCodes,targetID)
	delete(api.s.KnownUsers,targetID)
	for listID,l:=range api.s.Lists{
		if l.OwnerID!=targetID&&l.Permissions[targetID]==""{continue}
		if l.OwnerID==targetID{l.OwnerID=u.ID;delete(l.Permissions,u.ID)}
		delete(l.Permissions,targetID);l.UpdatedAt=time.Now().UnixMilli();api.s.Lists[listID]=l
		api.emit("list.updated",recipients(l),map[string]any{"origin":origin(r,u),"list":publicList(l)})
		api.emit("list.deleted",map[string]bool{targetID:true},map[string]any{"origin":origin(r,u),"listId":listID})
	}
	if e:=api.s.save();e!=nil{write(w,500,map[string]string{"error":"save failed"});return}
	api.emit("household.updated",oldRecipients,map[string]any{"origin":origin(r,u),"household":householdView(h)})
	write(w,200,householdView(h))
}
func householdView(h Household)map[string]any{members:=[]map[string]any{};for _,m:=range h.Members{name:=strings.TrimSpace(m.User.FirstName+" "+m.User.LastName);members=append(members,map[string]any{"userId":m.User.ID,"displayName":name,"email":m.User.Email,"role":m.Role,"canInvite":m.Role=="owner"||m.CanInvite,"allowMultipleDevices":m.Role=="owner"||m.AllowMultipleDevices,"joinedAt":m.JoinedAt})};return map[string]any{"id":h.ID,"name":h.Name,"members":members,"createdAt":h.CreatedAt}}
func (api *API) members(w http.ResponseWriter){api.s.RLock();defer api.s.RUnlock();out:=[]User{};for _,u:=range api.s.KnownUsers{out=append(out,u)};write(w,200,out)}

func (api *API) households(w http.ResponseWriter,u User){api.s.RLock();defer api.s.RUnlock();out:=[]map[string]any{};for _,h:=range api.s.Households{if member(h,u.ID){out=append(out,householdView(h))}};write(w,200,out)}
func (api *API) createHousehold(w http.ResponseWriter,r *http.Request,u User){var in struct{Name string};if decode(w,r,&in)!=nil{return};if strings.TrimSpace(in.Name)==""{write(w,400,map[string]string{"error":"name required"});return};now:=time.Now().UnixMilli();h:=Household{ID:id(),Name:strings.TrimSpace(in.Name),Members:[]Member{{User:u,Role:"owner",CanInvite:true,AllowMultipleDevices:true,JoinedAt:now}},CreatedAt:now,UpdatedAt:now};api.s.Lock();api.s.Households[h.ID]=h;e:=api.s.save();api.s.Unlock();if e!=nil{write(w,500,map[string]string{"error":"save failed"});return};write(w,201,householdView(h))}
func permission(l List,userID string)string{if l.OwnerID==userID{return "edit"};return l.Permissions[userID]}
func publicList(l List)List{l.SuggestionEvents=nil;return l}
func (api *API) lists(w http.ResponseWriter,u User){api.s.RLock();defer api.s.RUnlock();out:=[]List{};for _,l:=range api.s.Lists{if permission(l,u.ID)!=""{out=append(out,publicList(l))}};write(w,200,out)}
func (api *API) createList(w http.ResponseWriter,r *http.Request,u User){var in List;if decode(w,r,&in)!=nil{return};api.s.Lock();defer api.s.Unlock();h,ok:=api.s.Households[in.HouseholdID];if !ok||!member(h,u.ID){write(w,403,map[string]string{"error":"not a household member"});return};in.ID=id();in.OwnerID=u.ID;in.Permissions=map[string]string{};in.ItemHistory=map[string]int{};in.SuggestionEvents=map[string]int64{};in.UpdatedAt=time.Now().UnixMilli();if in.Icon==""{in.Icon="🛒"};api.s.Lists[in.ID]=in;_ = api.s.save();api.emit("list.updated",recipients(in),map[string]any{"origin":origin(r,u),"list":publicList(in)});write(w,201,publicList(in))}
func (api *API) updateList(w http.ResponseWriter,r *http.Request,u User){idv:=strings.TrimPrefix(r.URL.Path,"/v1/lists/");var in List;if decode(w,r,&in)!=nil{return};api.s.Lock();defer api.s.Unlock();old,ok:=api.s.Lists[idv];if !ok{api.createAt(w,r,u,idv,in);return};if permission(old,u.ID)==""{write(w,404,map[string]string{"error":"list not found"});return};p:=permission(old,u.ID);if p=="view"{write(w,403,map[string]string{"error":"view-only permission"});return};if p=="check"&&!checkOnly(old,in){write(w,403,map[string]string{"error":"check permission only allows check state changes"});return};in.ID=idv;in.OwnerID=old.OwnerID;in.HouseholdID=old.HouseholdID;in.Permissions=old.Permissions;in.EveryonePermission=old.EveryonePermission;in.ItemHistory=old.ItemHistory;in.SuggestionEvents=old.SuggestionEvents;in.NudgedAt=old.NudgedAt;in.NudgedBy=old.NudgedBy;in.UpdatedAt=time.Now().UnixMilli();api.s.Lists[idv]=in;_ = api.s.save();api.emit("list.updated",recipients(in),map[string]any{"origin":origin(r,u),"list":publicList(in)});write(w,200,publicList(in))}
// createAt lets the client own the list id, so PUT is an idempotent upsert and a
// newly created list never has to be renumbered on the device. Caller holds the lock.
func (api *API) createAt(w http.ResponseWriter,r *http.Request,u User,idv string,in List){
	if len(idv)<8||len(idv)>64{write(w,400,map[string]string{"error":"invalid list id"});return}
	if _,deleted:=api.s.DeletedLists[idv];deleted{write(w,410,map[string]string{"error":"this list was permanently deleted"});return}
	h,ok:=api.s.Households[in.HouseholdID];if !ok||!member(h,u.ID){write(w,403,map[string]string{"error":"not a household member"});return}
	in.ID=idv;in.OwnerID=u.ID;in.Permissions=map[string]string{};in.ItemHistory=map[string]int{};in.SuggestionEvents=map[string]int64{}
	if in.Icon==""{in.Icon="🛒"}
	in.UpdatedAt=time.Now().UnixMilli();api.s.Lists[idv]=in;_ = api.s.save()
	api.emit("list.updated",recipients(in),map[string]any{"origin":origin(r,u),"list":publicList(in)})
	write(w,201,publicList(in))
}
func (api *API) removeSuggestion(w http.ResponseWriter,r *http.Request,u User){rest:=strings.TrimPrefix(r.URL.Path,"/v1/lists/");parts:=strings.SplitN(rest,"/suggestions/",2);if len(parts)!=2||strings.TrimSpace(parts[1])==""{write(w,400,map[string]string{"error":"suggestion key required"});return};idv,key:=parts[0],strings.ToLower(strings.TrimSpace(parts[1]));api.s.Lock();defer api.s.Unlock();l,ok:=api.s.Lists[idv];if !ok||permission(l,u.ID)==""{write(w,404,map[string]string{"error":"list not found"});return};if permission(l,u.ID)!="edit"{write(w,403,map[string]string{"error":"edit permission is required to remove suggestions"});return};delete(l.ItemHistory,key);l.UpdatedAt=time.Now().UnixMilli();api.s.Lists[idv]=l;_ = api.s.save();api.emit("list.updated",recipients(l),map[string]any{"origin":origin(r,u),"list":publicList(l)});write(w,200,publicList(l))}
func (api *API) resetSuggestions(w http.ResponseWriter,r *http.Request,u User){idv:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/lists/"),"/suggestions");api.s.Lock();defer api.s.Unlock();l,ok:=api.s.Lists[idv];if !ok||permission(l,u.ID)==""{write(w,404,map[string]string{"error":"list not found"});return};if permission(l,u.ID)!="edit"{write(w,403,map[string]string{"error":"edit permission is required to reset suggestions"});return};l.ItemHistory=map[string]int{};l.SuggestionEvents=map[string]int64{};l.UpdatedAt=time.Now().UnixMilli();api.s.Lists[idv]=l;_ = api.s.save();api.emit("list.updated",recipients(l),map[string]any{"origin":origin(r,u),"list":publicList(l)});write(w,200,publicList(l))}
func (api *API) recordSuggestion(w http.ResponseWriter,r *http.Request,u User){idv:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/lists/"),"/suggestions");var in struct{Name,EventID string};if decode(w,r,&in)!=nil{return};name:=strings.ToLower(strings.TrimSpace(in.Name));eventID:=strings.TrimSpace(in.EventID);if name==""||len(name)>120||len(eventID)<8||len(eventID)>128{write(w,400,map[string]string{"error":"name and a valid eventId are required"});return};api.s.Lock();defer api.s.Unlock();l,ok:=api.s.Lists[idv];if !ok||permission(l,u.ID)==""{write(w,404,map[string]string{"error":"list not found"});return};if permission(l,u.ID)!="edit"{write(w,403,map[string]string{"error":"edit permission is required to add suggestions"});return};if l.ItemHistory==nil{l.ItemHistory=map[string]int{}};if l.SuggestionEvents==nil{l.SuggestionEvents=map[string]int64{}};if _,seen:=l.SuggestionEvents[eventID];!seen{now:=time.Now().UnixMilli();l.ItemHistory[name]++;l.SuggestionEvents[eventID]=now;l.UpdatedAt=now;if len(l.SuggestionEvents)>2000{cutoff:=time.Now().Add(-90*24*time.Hour).UnixMilli();for event,at:=range l.SuggestionEvents{if at<cutoff{delete(l.SuggestionEvents,event)}}};api.s.Lists[idv]=l;_ = api.s.save();api.emit("list.updated",recipients(l),map[string]any{"origin":origin(r,u),"list":publicList(l)})};write(w,200,map[string]any{"itemHistory":l.ItemHistory})}
func checkOnly(old,next List)bool{if old.Title!=next.Title||old.Icon!=next.Icon||old.StoreCountryCode!=next.StoreCountryCode||old.StoreCountryName!=next.StoreCountryName||old.QuickAddRows!=next.QuickAddRows||old.MaxQuickSuggestions!=next.MaxQuickSuggestions||old.SuggestionIncludesMeasures!=next.SuggestionIncludesMeasures||len(old.Items)!=len(next.Items){return false};byID:=map[string]Item{};for _,i:=range old.Items{byID[i.ID]=i};for _,i:=range next.Items{o,ok:=byID[i.ID];if !ok||o.Name!=i.Name||o.Quantity!=i.Quantity||o.Size!=i.Size||o.Unit!=i.Unit{return false}};return true}
// nudgeList records a "Notify family" tap and pushes it to everyone the list is shared
// with, except the person who tapped. The tap is kept on the list as well as emitted, so
// a phone that only checks every few hours still finds it on its next look. Anyone who
// can see the list may nudge: asking for the shopping is not an owner-only act.
func (api *API) nudgeList(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/lists/"),"/nudge")
	api.s.Lock()
	l,ok:=api.s.Lists[idv]
	if !ok||permission(l,u.ID)==""{api.s.Unlock();write(w,404,map[string]string{"error":"list not found"});return}
	now:=time.Now().UnixMilli()
	// One nudge per list per minute. A second tap is answered, but does not re-notify.
	if now-l.NudgedAt<nudgeCooldownMillis{api.s.Unlock();write(w,200,publicList(l));return}
	l.NudgedAt=now;l.NudgedBy=u.ID;api.s.Lists[idv]=l;_ = api.s.save()
	to:=recipients(l);delete(to,u.ID)
	api.s.Unlock()
	api.emit("list.nudged",to,map[string]any{"origin":origin(r,u),"list":publicList(l)})
	write(w,200,publicList(l))
}
const nudgeCooldownMillis=60_000
func validPermissionLevel(p string)bool{return p=="view"||p=="check"||p=="edit"}
func (api *API) updatePermissions(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/lists/"),"/permissions")
	var in struct{Permissions map[string]string;EveryonePermission string}
	if decode(w,r,&in)!=nil{return}
	api.s.Lock();defer api.s.Unlock()
	l,ok:=api.s.Lists[idv]
	if !ok||l.OwnerID!=u.ID{write(w,403,map[string]string{"error":"only the list owner can change permissions"});return}
	for userID,p:=range in.Permissions{if _,known:=api.s.KnownUsers[userID];!known||userID==u.ID||!validPermissionLevel(p){write(w,400,map[string]string{"error":"invalid permission assignment"});return}}
	if in.EveryonePermission!=""&&!validPermissionLevel(in.EveryonePermission){write(w,400,map[string]string{"error":"invalid everyone permission"});return}
	was:=recipients(l)
	l.Permissions=in.Permissions;l.EveryonePermission=in.EveryonePermission;l.UpdatedAt=time.Now().UnixMilli()
	api.s.Lists[idv]=l;_ = api.s.save()
	now:=recipients(l);dropped:=map[string]bool{};for uid:=range was{if !now[uid]{dropped[uid]=true}}
	api.emit("list.updated",now,map[string]any{"origin":origin(r,u),"list":publicList(l)})
	if len(dropped)>0{api.emit("list.deleted",dropped,map[string]any{"origin":origin(r,u),"listId":idv})}
	write(w,200,publicList(l))
}
// subscribableLists lists lists across the caller's household(s) that they do not already
// have access to, so Family server settings can offer a one-tap join. An ordinary member only
// sees lists their owner opened to everyone; a family administrator sees every list in the
// household regardless, since an admin is expected to be able to take over any list, not just
// the ones somebody chose to share. A list stops appearing here the moment the member has
// access, whether by subscribing, by an individual grant from the owner, or by owning it.
func (api *API) subscribableLists(w http.ResponseWriter,u User){
	api.s.RLock();defer api.s.RUnlock()
	households:=map[string]bool{};for _,h:=range api.s.Households{if member(h,u.ID){households[h.ID]=true}}
	out:=[]map[string]any{}
	for _,l:=range api.s.Lists{
		if !households[l.HouseholdID]||permission(l,u.ID)!=""{continue}
		h:=api.s.Households[l.HouseholdID]
		admin:=isOwner(h,u.ID)
		if l.EveryonePermission==""&&!admin{continue}
		ownerName:="Family member"
		for _,m:=range h.Members{if m.User.ID==l.OwnerID{ownerName=strings.TrimSpace(m.User.FirstName+" "+m.User.LastName);if ownerName==""{ownerName=m.User.Email};break}}
		level:=l.EveryonePermission;if admin||level==""{level="edit"}
		out=append(out,map[string]any{"id":l.ID,"title":l.Title,"icon":l.Icon,"ownerName":ownerName,"permission":level})
	}
	write(w,200,out)
}
// subscribeToList grants the caller access to a list. An ordinary member may only join one
// its owner shared with everyone; a family administrator can subscribe to any list in the
// household and always lands on edit, so an admin is never locked out of a list somebody
// never chose to share.
func (api *API) subscribeToList(w http.ResponseWriter,r *http.Request,u User){
	idv:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/v1/lists/"),"/subscribe")
	api.s.Lock();defer api.s.Unlock()
	l,ok:=api.s.Lists[idv]
	if !ok{write(w,404,map[string]string{"error":"list not found"});return}
	h,exists:=api.s.Households[l.HouseholdID]
	if !exists||!member(h,u.ID){write(w,403,map[string]string{"error":"not a household member"});return}
	admin:=isOwner(h,u.ID)
	if l.EveryonePermission==""&&!admin{write(w,403,map[string]string{"error":"this list is not shared with everyone"});return}
	if permission(l,u.ID)==""{
		level:=l.EveryonePermission;if admin||level==""{level="edit"}
		if l.Permissions==nil{l.Permissions=map[string]string{}}
		l.Permissions[u.ID]=level;l.UpdatedAt=time.Now().UnixMilli()
		api.s.Lists[idv]=l;_ = api.s.save()
		api.emit("list.updated",recipients(l),map[string]any{"origin":origin(r,u),"list":publicList(l)})
	}
	write(w,200,publicList(l))
}
func member(h Household,id string)bool{for _,m:=range h.Members{if m.User.ID==id{return true}};return false}
func decode(w http.ResponseWriter,r *http.Request,v any)error{return decodeLimit(w,r,v,1<<20)}
// Backups are far larger than any other request body, so that route raises the cap;
// without this an oversized backup fails as "invalid JSON" instead of a size error.
func decodeLimit(w http.ResponseWriter,r *http.Request,v any,limit int64)error{defer r.Body.Close();e:=json.NewDecoder(http.MaxBytesReader(w,r.Body,limit)).Decode(v);if e!=nil{write(w,400,map[string]string{"error":"invalid JSON"})};return e}
func write(w http.ResponseWriter,status int,v any){w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func id()string{return fmt.Sprintf("%x",sha256.Sum256([]byte(fmt.Sprintf("%d-%d",time.Now().UnixNano(),os.Getpid()))))[:24]}
func loadPairingKey()string{if value:=strings.TrimSpace(os.Getenv("PAIRING_KEY"));value!=""{return value};if file:=os.Getenv("PAIRING_KEY_FILE");file!=""{if value,e:=os.ReadFile(file);e==nil{return strings.TrimSpace(string(value))}};return ""}
func main(){port:=env("PORT","8787");if len(os.Args)>1&&os.Args[1]=="healthcheck"{resp,e:=http.Get("http://127.0.0.1:"+port+"/health");if e!=nil||resp.StatusCode!=200{os.Exit(1)};return};// Single sign-on is optional: with no issuer configured the connector runs on device
	// enrolment alone, which is the point of not requiring anyone to host a provider.
	issuer:=strings.TrimSpace(os.Getenv("OIDC_ISSUER"));aud:=env("OIDC_AUDIENCE","tidyshop-android");var auth *Auth;if issuer!=""{auth=newAuth(issuer,aud);log.Printf("single sign-on enabled for %s",issuer)}else{log.Print("single sign-on disabled; devices enrol with an invite code")};pairingKey:=loadPairingKey();if len(pairingKey)<12{log.Fatal("PAIRING_KEY or PAIRING_KEY_FILE must provide at least 12 characters")};api:=&API{s:newStore(env("DATA_FILE","/data/state.json")),a:auth,hub:newHub(),pairingKey:pairingKey};srv:=&http.Server{Addr:":"+port,Handler:api,ReadHeaderTimeout:5*time.Second,ReadTimeout:15*time.Second,WriteTimeout:15*time.Second,IdleTimeout:60*time.Second};log.Printf("TidyShop-Connector listening on :%s",port);log.Fatal(srv.ListenAndServe())}
func env(k,d string)string{if v:=os.Getenv(k);v!=""{return v};return d}
