package ton

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestVerifyProof_ValidSignature(t *testing.T) {
	// Генерируем ключевую пару Ed25519
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pubKeyHex := hex.EncodeToString(pubKey)

	// Address: workchain=0, hash=32 bytes
	addrHash := make([]byte, 32)
	for i := range addrHash {
		addrHash[i] = byte(i)
	}
	workchain := int32(0)

	proof := Proof{
		Timestamp: time.Now().Unix(),
		Domain: ProofDomain{
			LengthBytes: len("test.example.com"),
			Value:       "test.example.com",
		},
		Payload: "test-nonce-12345",
	}

	// Собираем message по спецификации
	message := []byte(TonProofPrefix)

	wcBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(wcBytes, uint32(workchain))
	message = append(message, wcBytes...)
	message = append(message, addrHash...)

	domainLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(domainLenBytes, uint32(proof.Domain.LengthBytes))
	message = append(message, domainLenBytes...)
	message = append(message, []byte(proof.Domain.Value)...)

	tsBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(tsBytes, uint64(proof.Timestamp))
	message = append(message, tsBytes...)
	message = append(message, []byte(proof.Payload)...)

	// signature_message = 0xffff ++ "ton-connect" ++ sha256(message)
	msgHash := sha256.Sum256(message)
	signatureMessage := []byte{0xff, 0xff}
	signatureMessage = append(signatureMessage, []byte(TonConnectPrefix)...)
	signatureMessage = append(signatureMessage, msgHash[:]...)

	finalHash := sha256.Sum256(signatureMessage)

	// Подписываем
	sig := ed25519.Sign(privKey, finalHash[:])
	proof.Signature = hex.EncodeToString(sig)

	// Верифицируем
	err = VerifyProof(pubKeyHex, addrHash, workchain, proof, []string{"test.example.com"})
	if err != nil {
		t.Fatalf("expected valid proof, got error: %v", err)
	}
}

func TestVerifyProof_ExpiredTimestamp(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(nil)

	proof := Proof{
		Timestamp: time.Now().Add(-10 * time.Minute).Unix(),
		Domain:    ProofDomain{LengthBytes: 4, Value: "test"},
		Payload:   "nonce",
		Signature: hex.EncodeToString(make([]byte, 64)),
	}

	err := VerifyProof(hex.EncodeToString(pubKey), make([]byte, 32), 0, proof, nil)
	if err == nil {
		t.Fatal("expected error for expired proof")
	}
}

func TestVerifyProof_WrongDomain(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(nil)

	proof := Proof{
		Timestamp: time.Now().Unix(),
		Domain:    ProofDomain{LengthBytes: 8, Value: "evil.com"},
		Payload:   "nonce",
		Signature: hex.EncodeToString(make([]byte, 64)),
	}

	err := VerifyProof(hex.EncodeToString(pubKey), make([]byte, 32), 0, proof, []string{"good.com"})
	if err == nil {
		t.Fatal("expected error for wrong domain")
	}
}

func TestVerifyProof_InvalidSignature(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(nil)

	proof := Proof{
		Timestamp: time.Now().Unix(),
		Domain:    ProofDomain{LengthBytes: 4, Value: "test"},
		Payload:   "nonce",
		Signature: hex.EncodeToString(make([]byte, 64)), // нулевая подпись
	}

	err := VerifyProof(hex.EncodeToString(pubKey), make([]byte, 32), 0, proof, nil)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestParseRawAddress(t *testing.T) {
	tests := []struct {
		input string
		wc    int32
		valid bool
	}{
		{"0:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", 0, true},
		{"-1:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", -1, true},
		{"invalid", 0, false},
		{"0:short", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			wc, hash, err := ParseRawAddress(tt.input)
			if tt.valid {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
				if wc != tt.wc {
					t.Errorf("workchain = %d, want %d", wc, tt.wc)
				}
				if len(hash) != 32 {
					t.Errorf("hash len = %d, want 32", len(hash))
				}
			} else {
				if err == nil {
					t.Fatal("expected error for invalid address")
				}
			}
		})
	}
}
