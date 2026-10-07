package ldap_db

import (
	"crypto/x509"
	"fmt"
	"os"
)

func readCertificate(certPath string) (*x509.CertPool, error) {
	const op string = "ldap_db.readCertificate"
	rootCertPool := x509.NewCertPool()
	pem, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read certificate: %w", op, err)
	}

	ok := rootCertPool.AppendCertsFromPEM(pem)
	if !ok {
		return nil, fmt.Errorf("%s: failed to append certificate to pool", op)
	}

	return rootCertPool, nil
}
