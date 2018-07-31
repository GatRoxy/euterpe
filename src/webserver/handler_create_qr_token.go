package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gbrlsnchs/jwt"
	"github.com/skip2/go-qrcode"

	"github.com/ironsmile/httpms/src/config"
)

func NewCreateQRTokenHandler(needsAuth bool, auth config.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		qrConts := struct {
			Software string `json:"software"`
			Token    string `json:"token,omitempty"`
			Address  string `json:"address"`
		}{
			Software: "httpms",
			Address:  fmt.Sprintf("http://%s", r.Host),
		}

		if needsAuth {
			tokenOpts := &jwt.Options{
				Timestamp:      true,
				ExpirationTime: time.Now().Add(6 * 31 * 24 * time.Hour),
			}
			token, err := jwt.Sign(jwt.HS256(auth.Secret), tokenOpts)
			if err != nil {
				errMsg := fmt.Sprintf("Error generating token: %s.", err)
				http.Error(w, errMsg, http.StatusInternalServerError)
				return
			}

			qrConts.Token = token
		}

		qrBytes, err := json.Marshal(&qrConts)
		if err != nil {
			errMsg := fmt.Sprintf("Error JSON encoding token: %s.", err)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		qr, err := qrcode.New(string(qrBytes), qrcode.Medium)
		if err != nil {
			errMsg := fmt.Sprintf("Error creating QR token: %s.", err)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		if err := qr.Write(500, w); err != nil {
			errMsg := fmt.Sprintf("Error writing out qr token: %s.", err)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	})
}
