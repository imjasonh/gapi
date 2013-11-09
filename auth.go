package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/goauth2/oauth/jwt"
	"encoding/gob"
	"encoding/json"
	"net/http"
)

const tokensFile = "~tokens.gob"

func authStart() {
	args := endpointFs.Args()
	if len(args) != 3 {
		fmt.Println("Invalid arguments, must provide API and method, e.g.:")
		fmt.Println("  gapi auth.start <api> <method>")
		return
	}
	api := loadAPI(args[1])
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		fmt.Println("Couldn't load API ", args[1])
		return
	}
	m := findMethod(args[2], *api)
	if m.Scopes == nil {
		fmt.Println("Method doesn't require auth")
		return
	}
	oauthConfig.Scope = strings.Join(m.Scopes, " ")
	fmt.Println("Open a browser and visit the following URL:")
	fmt.Println(oauthConfig.AuthCodeURL(""))
	fmt.Println("Then run the following command with the resulting auth code:")
	fmt.Println("gapi auth.finish <code>")
}
func authFinish() {
	args := endpointFs.Args()
	if len(args) != 2 {
		fmt.Println("Invalid arguments, must provide code, e.g.:")
		fmt.Println("  gapi auth.finish <code>")
		return
	}
	code := args[1]
	t := &oauth.Transport{Config: oauthConfig}
	tok, err := t.Exchange(code)
	maybeFatal("error exchanging code", err)
	toks, err := loadTokens()
	maybeFatal("error loading tokens", err)
	inf, err := getTokenInfo(tok.AccessToken)
	maybeFatal("error getting tokeninfo", err)
	toks.Tok[inf.Scope] = token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
	}
	err = toks.save()
	maybeFatal("error saving tokens", err)
	fmt.Println("Token saved")
}
func authPrint() {
	args := endpointFs.Args()
	if len(args) != 3 {
		fmt.Println("Invalid arguments, must provide API and method, e.g.:")
		fmt.Println("  gapi auth.print <api> <method>")
		return
	}
	api := loadAPI(args[1])
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", args[1])
	}
	m := findMethod(args[2], *api)
	toks, err := loadTokens()
	maybeFatal("error loading tokens", err)
	if tok, found := toks.Tok[strings.Join(m.Scopes, " ")]; found {
		// TODO: If necessary, refresh, store, and print the new token.
		fmt.Println(tok.AccessToken)
	} else {
		fmt.Println("No token found. Run the following command to store a token:")
		fmt.Println("gapi auth.start", api.Name, m.ID[len(api.Name)+1:])
	}
}
func authRevoke() {
	args := endpointFs.Args()
	if len(args) != 3 {
		fmt.Println("Invalid arguments, must provide API and method, e.g.:")
		fmt.Println("  gapi auth.print <api> <method>")
		return
	}
	api := loadAPI(args[1])
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", args[1])
	}
	m := findMethod(args[2], *api)
	toks, err := loadTokens()
	maybeFatal("error loading tokens", err)
	if tok, found := toks.Tok[strings.Join(m.Scopes, " ")]; found {
		_, err := http.Get("https://accounts.google.com/o/oauth2/revoke?token=" + tok)
		maybeFatal("error revoking token", err)
		delete(toks.Tok, strings.Join(m.Scopes, " "))
		toks.save()
	}
}

type tokenInfo struct {
	Scope      string
	ExpiresIn  int    `json:"expires_in"`
	AccessType string `json:"access_type"`
}

func (inf tokenInfo) expired() bool {
	return inf.ExpiresIn < 0
}
func getTokenInfo(tok string) (*tokenInfo, error) {
	r, err := http.Post("https://www.googleapis.com/oauth2/v2/tokeninfo?access_token="+tok, "", nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	var info tokenInfo
	err = json.NewDecoder(r.Body).Decode(&info)
	return &info, err
}

type tokens struct {
	Tok map[string]token
}

type token struct {
	AccessToken, RefreshToken string
}

func loadTokens() (*tokens, error) {
	fi, err := os.Stat(tokensFile)
	if err == os.ErrNotExist || err == io.EOF {
		return &tokens{make(map[string]token)}, nil
	} else if err != nil {
		return &tokens{make(map[string]token)}, nil
	}
	if fi.Size() == 0 {
		return &tokens{make(map[string]token)}, nil
	}
	f, err := os.Open(tokensFile)
	if err != nil && err != io.EOF {
		return &tokens{make(map[string]token)}, nil
	}
	defer f.Close()
	var t tokens
	_ = gob.NewDecoder(f).Decode(&t)
	return &t, nil
}

func (t *tokens) save() error {
	f, err := os.Create(tokensFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(t)
}

func accessTokenFromPemFile(scope string) string {
	secretBytes, err := ioutil.ReadFile(*flagSecrets)
	maybeFatal("error reading secrets file:", err)
	var config struct {
		Web struct {
			ClientEmail string `json:"client_email"`
			TokenURI    string `json:"token_uri"`
		}
	}
	err = json.Unmarshal(secretBytes, &config)
	maybeFatal("error unmarshalling secrets:", err)

	keyBytes, err := ioutil.ReadFile(*flagPem)
	maybeFatal("error reading private key file:", err)

	// Craft the ClaimSet and JWT token.
	t := jwt.NewToken(config.Web.ClientEmail, scope, keyBytes)
	t.ClaimSet.Aud = config.Web.TokenURI

	// We need to provide a client.
	c := &http.Client{}

	// Get the access token.
	o, err := t.Assert(c)
	maybeFatal("assertion error:", err)

	return o.AccessToken
}
