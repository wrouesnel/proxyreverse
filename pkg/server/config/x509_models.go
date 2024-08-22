package config

import (
	"crypto/sha256"
	"crypto/x509"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/wrouesnel/proxyreverse/pkg/certutils"
	"go.uber.org/zap"
)

const (
	TLSCertificatePoolMaxNonFileEntryReturn int    = 50
	TLSCACertsSystem                        string = "system"
)

var (
	ErrInvalidInputType = errors.New("invalid input type for decoder")
	ErrInvalidPEMFile   = errors.New("PEM file could not be added to certificate pool")
)

// TLSCertificateMap encodes a list of certificates and stores them in a hashmap
// for easy lookups. It is similar to the standard library CertPool.
type Sum224 [sha256.Size224]byte
type TLSCertificateMap struct {
	certMap  map[Sum224]*x509.Certificate
	original []string
}

func (t *TLSCertificateMap) GetCerts() []*x509.Certificate {
	if t.certMap == nil {
		return []*x509.Certificate{}
	}

	return lo.MapToSlice(t.certMap, func(_ Sum224, v *x509.Certificate) *x509.Certificate {
		return v
	})
}

func (t *TLSCertificateMap) HasCert(cert *x509.Certificate) bool {
	if t.certMap == nil {
		return false
	}
	_, found := t.certMap[sha256.Sum224(cert.Raw)]
	return found
}

func (t *TLSCertificateMap) HashCert(cert *x509.Certificate) Sum224 {
	return sha256.Sum224(cert.Raw)
}

// AddCert adds a certificate to a pool.
func (t *TLSCertificateMap) AddCert(cert *x509.Certificate) error {
	if t.certMap == nil {
		t.certMap = make(map[Sum224]*x509.Certificate)
	}

	// Just allow this. It's not fatal.
	if cert == nil {
		return nil
	}

	// Hash the certificate
	rawSum224 := sha256.Sum224(cert.Raw)

	// Check that the certificate isn't being added twice.
	if _, found := t.certMap[rawSum224]; found {
		return nil
	}

	t.certMap[rawSum224] = cert
	return nil
}

// MapStructureDecode implements unmarshalling for TLSCertificateMap
// nolint: funlen,cyclop
func (t *TLSCertificateMap) MapStructureDecode(input interface{}) error {
	// Get the slice
	interfaceSlice, ok := input.([]interface{})
	if !ok {
		return errors.Wrapf(ErrInvalidInputType, "expected []string got %T", input)
	}

	// Get the strings
	strErrors := lo.Map(interfaceSlice, func(t interface{}, i int) lo.Tuple2[string, bool] {
		strValue, ok := t.(string)
		return lo.T2(strValue, ok)
	})

	// Extract errors
	err := lo.Reduce[lo.Tuple2[string, bool], error](strErrors, func(r error, t lo.Tuple2[string, bool], _ int) error {
		// Return the first error we got if its there
		if r != nil {
			return r
		}
		_, ok := lo.Unpack2(t)
		if !ok {
			return errors.Wrapf(ErrInvalidInputType, "invalid input type in certificate list")
		}
		return nil
	}, nil)
	if err != nil {
		return errors.Wrapf(err, "TLSCertificatePool.MapStructureDecode")
	}

	// Flatten the valid list out to []string
	caCertSpecEntries := lo.Map(strErrors, func(t lo.Tuple2[string, bool], _ int) string {
		value, _ := lo.Unpack2(t)
		return value
	})

	for idx, entry := range caCertSpecEntries {
		var pem []byte
		itemSample := ""
		if _, err := os.Stat(entry); err == nil {
			// Is a file
			pem, err = ioutil.ReadFile(entry)
			if err != nil {
				return errors.Wrapf(err, "could not read certificate file: %s", entry)
			}
			itemSample = entry
		} else {
			pem = []byte(entry)
			if len(entry) < TLSCertificatePoolMaxNonFileEntryReturn {
				itemSample = entry
			} else {
				itemSample = entry[:TLSCertificatePoolMaxNonFileEntryReturn]
			}
		}

		certificates, err := certutils.LoadCertificatesFromPem(pem)
		if err != nil {
			return errors.Wrapf(ErrInvalidPEMFile, "failed at item %v: %v", idx, itemSample)
		}

		for idx, cert := range certificates {
			if err := t.AddCert(cert); err != nil {
				return errors.Wrapf(ErrInvalidPEMFile, "failed at item %v: %v", idx, itemSample)
			}
		}
	}

	t.original = caCertSpecEntries
	return nil
}

// TLSCertificatePool is our custom type for decoding a certificate pool out of
// YAML.
type TLSCertificatePool struct {
	*x509.CertPool
	original []string
}

// MapStructureDecode implements the yaml.Unmarshaler interface for tls_cacerts.
//
//nolint:funlen,cyclop
func (t *TLSCertificatePool) MapStructureDecode(input interface{}) error {
	// Get the slice
	interfaceSlice, ok := input.([]interface{})
	if !ok {
		return errors.Wrapf(ErrInvalidInputType, "expected []string got %T", input)
	}

	// Get the strings
	strErrors := lo.Map(interfaceSlice, func(t interface{}, i int) lo.Tuple2[string, bool] {
		strValue, ok := t.(string)
		return lo.T2(strValue, ok)
	})

	// Extract errors
	err := lo.Reduce[lo.Tuple2[string, bool], error](strErrors, func(r error, t lo.Tuple2[string, bool], _ int) error {
		// Return the first error we got if its there
		if r != nil {
			return r
		}
		_, ok := lo.Unpack2(t)
		if !ok {
			return errors.Wrapf(ErrInvalidInputType, "invalid input type in certificate list")
		}
		return nil
	}, nil)
	if err != nil {
		return errors.Wrapf(err, "TLSCertificatePool.MapStructureDecode")
	}

	// Flatten the valid list out to []string
	caCertSpecEntries := lo.Map(strErrors, func(t lo.Tuple2[string, bool], _ int) string {
		value, _ := lo.Unpack2(t)
		return value
	})

	// Prescan to check for system cert package request
	t.CertPool = nil
	for _, entry := range caCertSpecEntries {
		if entry == TLSCACertsSystem {
			rootCAs, err := x509.SystemCertPool()
			if err != nil {
				zap.L().Warn("could not fetch system certificate pool", zap.Error(err))
				rootCAs = x509.NewCertPool()
			}
			t.CertPool = rootCAs
			break
		}
	}

	if t.CertPool == nil {
		t.CertPool = x509.NewCertPool()
	}

	//nolint:nestif
	for idx, entry := range caCertSpecEntries {
		var pem []byte
		itemSample := ""
		if entry == TLSCACertsSystem {
			// skip - handled above
			continue
		} else if _, err := os.Stat(entry); err == nil {
			// Is a file
			pem, err = ioutil.ReadFile(entry)
			if err != nil {
				return errors.Wrapf(err, "could not read certificate file: %s", entry)
			}
			itemSample = entry
		} else {
			pem = []byte(entry)
			if len(entry) < TLSCertificatePoolMaxNonFileEntryReturn {
				itemSample = entry
			} else {
				itemSample = entry[:TLSCertificatePoolMaxNonFileEntryReturn]
			}
		}
		if ok := t.CertPool.AppendCertsFromPEM(pem); !ok {
			return errors.Wrapf(ErrInvalidPEMFile, "failed at item %v: %v", idx, itemSample)
		}
	}

	t.original = caCertSpecEntries

	return nil
}
