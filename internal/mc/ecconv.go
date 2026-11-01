package mc

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
)

func DERtoRawPoint(der []byte) ([]byte, error) {
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {

		if len(der) == 97 && der[0] == 4 {
			return der, nil
		}
		return nil, fmt.Errorf("解析服务器公钥 DER 失败: %w", err)
	}
	e, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("不是 EC 公钥")
	}
	xb := make([]byte, 48)
	yb := make([]byte, 48)
	e.X.FillBytes(xb)
	e.Y.FillBytes(yb)
	raw := make([]byte, 0, 97)
	raw = append(raw, 4)
	raw = append(raw, xb...)
	raw = append(raw, yb...)
	return raw, nil
}

func keyPairFromECDSA(k *ecdsa.PrivateKey) (*KeyPair, error) {

	d := make([]byte, 48)
	k.D.FillBytes(d)
	priv, err := ecdh.P384().NewPrivateKey(d)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Priv: priv}, nil
}
