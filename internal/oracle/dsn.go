package oracle

import (
	"fmt"
	"net/url"
)

// buildOracleDSN arma un DSN Oracle escapando correctamente usuario y clave.
// Esto evita fallos con caracteres especiales o acentos en las credenciales.
func buildOracleDSN(username, password, host, service string) string {
	u := &url.URL{
		Scheme: "oracle",
		User:   url.UserPassword(username, password),
		Host:   fmt.Sprintf("%s:1521", host),
		Path:   "/" + service,
	}
	return u.String()
}
