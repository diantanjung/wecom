package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const minSecretKeySize = 32

// JWTMaker is a JSON Web Token maker
type JWTMaker struct {
	secretKey string
}

// NewJWTMaker creates a new JWTMaker
func NewJWTMaker(secretKey string) (Maker, error) {
	if len(secretKey) < minSecretKeySize {
		return nil, fmt.Errorf("invalid key size: must be at least %d characters", minSecretKeySize)
	}
	return &JWTMaker{secretKey}, nil
}

// CreateToken creates a new token for a specific username and duration
func (maker *JWTMaker) CreateToken(userID int64, username string, duration time.Duration) (string, error) {
	payload, err := NewPayload(userID, username, duration)
	if err != nil {
		return "", err
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	return jwtToken.SignedString([]byte(maker.secretKey))
}

// VerifyToken checks if the token is valid or not
// func (maker *JWTMaker) VerifyToken(token string) (*Payload, error) {
// 	keyFunc := func(token *jwt.Token) (interface{}, error) {
// 		_, ok := token.Method.(*jwt.SigningMethodHMAC)
// 		if !ok {
// 			return nil, ErrInvalidToken
// 		}
// 		return []byte(maker.secretKey), nil
// 	}

// 	jwtToken, err := jwt.ParseWithClaims(token, &Payload{}, keyFunc)
// 	if err != nil {
// 		verr, ok := err.(*jwt.ValidationError)
// 		if ok && errors.Is(verr.Inner, ErrExpiredToken) {
// 			return nil, ErrExpiredToken
// 		}
// 		return nil, ErrInvalidToken
// 	}

// 	payload, ok := jwtToken.Claims.(*Payload)
// 	if !ok {
// 		return nil, ErrInvalidToken
// 	}

// 	return payload, nil
// }

func (maker *JWTMaker) VerifyToken(tokenString string) (*Payload, error) {
	if len(tokenString) > 50 {
		claimsStruct := Payload{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			&claimsStruct,
			func(token *jwt.Token) (interface{}, error) {
				pem, err := getGooglePublicKey(fmt.Sprintf("%s", token.Header["kid"]))
				if err != nil {
					return nil, err
				}
				key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pem))
				if err != nil {
					return nil, err
				}
				return key, nil
			},
		)
		if err != nil {
			return nil, err
		}

		claims, ok := token.Claims.(*Payload)
		if !ok {
			return nil, errors.New("Invalid Google JWT")
		}

		if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
			return nil, errors.New("iss is invalid")
		}

		if claims.Audience != os.Getenv("GOOGLE_CLIENT_ID") {
			return nil, errors.New("aud is invalid")
		}

		if claims.ExpiresAt < time.Now().UTC().Unix() {
			return nil, errors.New("JWT is expired")
		}

		return claims, nil
	} else {
		type githubDataResponse struct {
			Username string `json:"login"`
		}
	
		githubDataResp, err := getGithubData(tokenString)
		var ghdataresp githubDataResponse
	
		json.Unmarshal(githubDataResp, &ghdataresp)

		payload, err := NewPayload(99, ghdataresp.Username, 60)
		if err != nil {
			return nil, err
		}
		return payload, nil
	}

}

func getGithubData(accessToken string) ([]byte, error) {
    // Get request to a set URL
    req, reqerr := http.NewRequest("GET","https://api.github.com/user",nil)
    if reqerr != nil {
        return nil, reqerr
    }

    // Set the Authorization header before sending the request
    // Authorization: token XXXXXXXXXXXXXXXXXXXXXXXXXXX
    authorizationHeaderValue := fmt.Sprintf("token %s", accessToken)
    req.Header.Set("Authorization", authorizationHeaderValue)

    // Make the request
    resp, resperr := http.DefaultClient.Do(req)
    if resperr != nil {
        return nil, resperr
    }

    // Read the response as a byte slice
    respbody, _ := ioutil.ReadAll(resp.Body)

    // Convert byte slice to string and return
    return respbody,nil
}

func getGooglePublicKey(keyID string) (string, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v1/certs")
	if err != nil {
		return "", err
	}
	dat, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	myResp := map[string]string{}
	err = json.Unmarshal(dat, &myResp)
	if err != nil {
		return "", err
	}
	key, ok := myResp[keyID]
	if !ok {
		return "", errors.New("key not found")
	}
	return key, nil
}
