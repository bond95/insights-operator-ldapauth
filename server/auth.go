package server

import (
	"context"
	"encoding/json"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	auth "github.com/redhatinsights/insights-operator-ldapauth/auth"
	u "github.com/redhatinsights/insights-operator-ldapauth/utils"
	"net/http"
	"os"
	"strings"
)

func login(writer http.ResponseWriter, request *http.Request, ldap string) {
	account := &auth.Account{}
	err := json.NewDecoder(request.Body).Decode(account) //decode the request body into struct and failed if any error occur
	if err != nil {
		status := u.BuildResponse("Invalid request")
		u.SendError(writer, status)
		return
	}

	resp := auth.Authenticate(account.Login, account.Password, ldap)
	u.SendResponse(writer, resp)
}

var JwtAuthentication = func(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		notAuth := []string{API_PREFIX + "login"} //List of endpoints that doesn't require auth
		requestPath := r.URL.Path                 //current request path

		//check if request does not need authentication, serve the request if it doesn't need it
		for _, value := range notAuth {

			if value == requestPath {
				next.ServeHTTP(w, r)
				return
			}
		}

		response := make(map[string]interface{})
		tokenHeader := r.Header.Get("Authorization") //Grab the token from the header

		if tokenHeader == "" { //Token is missing, returns with error code 403 Unauthorized
			response = u.BuildResponse("Missing auth token")
			u.SendForbidden(w, response)
			return
		}

		splitted := strings.Split(tokenHeader, " ") //The token normally comes in format `Bearer {token-body}`, we check if the retrieved token matched this requirement
		if len(splitted) != 2 {
			response = u.BuildResponse("Invalid/Malformed auth token")
			u.SendForbidden(w, response)
			return
		}

		tokenPart := splitted[1] //Grab the token part, what we are truly interested in
		tk := &auth.Token{}

		token, err := jwt.ParseWithClaims(tokenPart, tk, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("token_password")), nil
		})

		if err != nil { //Malformed token, returns with http code 403 as usual
			response = u.BuildResponse("Malformed authentication token")
			u.SendForbidden(w, response)
			return
		}

		if !token.Valid { //Token is invalid, maybe not signed on this server
			response = u.BuildResponse("Token is not valid.")
			u.SendForbidden(w, response)
			return
		}

		//Everything went well, proceed with the request and set the caller to the user retrieved from the parsed token
		fmt.Sprintf("User %", tk.Login) //Useful for monitoring
		ctx := context.WithValue(r.Context(), "user", tk.Login)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r) //proceed in the middleware chain!
	})
}
