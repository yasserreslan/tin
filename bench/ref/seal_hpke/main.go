package main

import (
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func hx(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
func vector(curve ecdh.Curve, sk, enc, ct string, aead hpke.AEAD) {
	priv, err := curve.NewPrivateKey(hx(sk))
	if err != nil {
		panic(err)
	}
	key, err := hpke.NewDHKEMPrivateKey(priv)
	if err != nil {
		panic(err)
	}
	r, err := hpke.NewRecipient(hx(enc), key, hpke.HKDFSHA256(), aead, hx("4f6465206f6e2061204772656369616e2055726e"))
	if err != nil {
		panic(err)
	}
	pt, err := r.Open(hx("436f756e742d30"), hx(ct))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", pt)
}
func generated(curve ecdh.Curve, aead hpke.AEAD) {
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	pub, err := hpke.NewDHKEMPublicKey(priv.PublicKey())
	if err != nil {
		panic(err)
	}
	key, err := hpke.NewDHKEMPrivateKey(priv)
	if err != nil {
		panic(err)
	}
	for n := 0; n < 8; n++ {
		info, aad, msg := []byte{byte(n), 'I', 'N', 'F', 'O'}, []byte{byte(n), 'A', 'A', 'D'}, make([]byte, n*3)
		for i := range msg {
			msg[i] = byte(i*37 + n)
		}
		enc, sender, err := hpke.NewSender(pub, hpke.HKDFSHA256(), aead, info)
		if err != nil {
			panic(err)
		}
		ct, err := sender.Seal(aad, msg)
		if err != nil {
			panic(err)
		}
		r, err := hpke.NewRecipient(enc, key, hpke.HKDFSHA256(), aead, info)
		if err != nil {
			panic(err)
		}
		opened, err := r.Open(aad, ct)
		if err != nil || string(opened) != string(msg) {
			panic("round trip")
		}
		for _, x := range []struct{ e, i, a, c []byte }{{enc, []byte{0}, aad, ct}, {enc, info, []byte{0}, ct}, {enc, info, aad, append([]byte{ct[0] ^ 1}, ct[1:]...)}, {[]byte{0}, info, aad, ct}} {
			r, err := hpke.NewRecipient(x.e, key, hpke.HKDFSHA256(), aead, x.i)
			if err == nil {
				_, err = r.Open(x.a, x.c)
			}
			if err == nil {
				panic("accepted changed input")
			}
		}
	}
	fmt.Println("roundtrip")
}
func main() {
	vector(ecdh.X25519(), "4612c550263fc8ad58375df3f557aac531d26850903e55a9f23f21d8534e8ac8", "37fda3567bdbd628e88668c3c8d7e97d1d1253b6d4ea6d44c150f741f1bf4431", "f938558b5d72f1a23810b4be2ab4f84331acc02fc97babc53a52ae8218a355a96d8770ac83d07bea87e13c512a", hpke.AES128GCM())
	vector(ecdh.X25519(), "8057991eef8f1f1af18f4a9491d16a1ce333f695d4db8e38da75975c4478e0fb", "1afa08d3dec047a643885163f1180476fa7ddb54c6a8029ea33f95796bf2ac4a", "1c5250d8034ec2b784ba2cfd69dbdb8af406cfe3ff938e131f0def8c8b60b4db21993c62ce81883d2dd1b51a28", hpke.ChaCha20Poly1305())
	vector(ecdh.P256(), "f3ce7fdae57e1a310d87f1ebbde6f328be0a99cdbcadf4d6589cf29de4b8ffd2", "04a92719c6195d5085104f469a8b9814d5838ff72b60501e2c4466e5e67b325ac98536d7b61a1af4b78e5b7f951c0900be863c403ce65c9bfcb9382657222d18c4", "5ad590bb8baa577f8619db35a36311226a896e7342a6d836d8b7bcd2f20b6c7f9076ac232e3ab2523f39513434", hpke.AES128GCM())
	vector(ecdh.P256(), "a4d1c55836aa30f9b3fbb6ac98d338c877c2867dd3a77396d13f68d3ab150d3b", "04c07836a0206e04e31d8ae99bfd549380b072a1b1b82e563c935c095827824fc1559eac6fb9e3c70cd3193968994e7fe9781aa103f5b50e934b5b2f387e381291", "6469c41c5c81d3aa85432531ecf6460ec945bde1eb428cb2fedf7a29f5a685b4ccb0d057f03ea2952a27bb458b", hpke.ChaCha20Poly1305())
	for _, curve := range []ecdh.Curve{ecdh.X25519(), ecdh.P256()} {
		for _, aead := range []hpke.AEAD{hpke.AES128GCM(), hpke.AES256GCM(), hpke.ChaCha20Poly1305()} {
			generated(curve, aead)
		}
	}
}
