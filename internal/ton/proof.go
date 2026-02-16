package ton

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// TonProofPrefix — фиксированный префикс для TON Proof по спецификации TON Connect.
	// https://docs.ton.org/develop/dapps/ton-connect/sign#checking-ton_proof-on-server-side
	TonProofPrefix = "ton-proof-item-v2/"

	// TonConnectPrefix — префикс перед SHA256 хешем сообщения.
	TonConnectPrefix = "ton-connect"

	// MaxProofAge — максимальный возраст proof (защита от replay).
	MaxProofAge = 5 * time.Minute
)

// ProofData содержит данные из TON Connect ton_proof.
type ProofData struct {
	// Address — raw address: workchain (int32) + hash (32 bytes)
	Address    string `json:"address"`
	Network    string `json:"network"` // "-239" = mainnet, "-3" = testnet
	PublicKey  string `json:"public_key"` // hex
	Proof      Proof  `json:"proof"`
	StateInit  string `json:"state_init,omitempty"` // base64 BOC
}

type Proof struct {
	Timestamp int64       `json:"timestamp"`
	Domain    ProofDomain `json:"domain"`
	Payload   string      `json:"payload"`   // наш nonce
	Signature string      `json:"signature"` // base64
}

type ProofDomain struct {
	LengthBytes int    `json:"lengthBytes"`
	Value       string `json:"value"`
}

// VerifyProof проверяет TON Proof подпись.
//
// Алгоритм (по спецификации TON Connect):
// 1. message = "ton-proof-item-v2/" ++ address_workchain(4 bytes) ++ address_hash(32 bytes)
//              ++ domain_len(4 bytes LE) ++ domain ++ timestamp(8 bytes LE) ++ payload
// 2. signature_message = 0xffff ++ "ton-connect" ++ sha256(message)
// 3. Verify Ed25519(public_key, sha256(signature_message), signature)
func VerifyProof(pubKeyHex string, address []byte, workchain int32, proof Proof, allowedDomains []string) error {
	// 1. Проверяем timestamp
	proofTime := time.Unix(proof.Timestamp, 0)
	if time.Since(proofTime) > MaxProofAge {
		return fmt.Errorf("proof expired: %s old", time.Since(proofTime).Round(time.Second))
	}
	if proofTime.After(time.Now().Add(1 * time.Minute)) {
		return fmt.Errorf("proof timestamp is in the future")
	}

	// 2. Проверяем domain
	if !isDomainAllowed(proof.Domain.Value, allowedDomains) {
		return fmt.Errorf("domain %q not in allowed list", proof.Domain.Value)
	}

	// 3. Декодируем public key
	pubKey, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("invalid public key hex: %w", err)
	}
	if len(pubKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: %d", len(pubKey))
	}

	// 4. Декодируем signature
	sig, err := hex.DecodeString(proof.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size: %d", len(sig))
	}

	// 5. Собираем message
	//    "ton-proof-item-v2/" ++ workchain(4 LE) ++ address_hash(32) ++
	//    domain_len(4 LE) ++ domain ++ timestamp(8 LE) ++ payload
	message := []byte(TonProofPrefix)

	wcBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(wcBytes, uint32(workchain))
	message = append(message, wcBytes...)

	message = append(message, address...)

	domainLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(domainLenBytes, uint32(proof.Domain.LengthBytes))
	message = append(message, domainLenBytes...)
	message = append(message, []byte(proof.Domain.Value)...)

	tsBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(tsBytes, uint64(proof.Timestamp))
	message = append(message, tsBytes...)

	message = append(message, []byte(proof.Payload)...)

	// 6. signature_message = 0xffff ++ "ton-connect" ++ sha256(message)
	msgHash := sha256.Sum256(message)

	signatureMessage := []byte{0xff, 0xff}
	signatureMessage = append(signatureMessage, []byte(TonConnectPrefix)...)
	signatureMessage = append(signatureMessage, msgHash[:]...)

	// 7. Верифицируем: ed25519.Verify(pubKey, sha256(signatureMessage), sig)
	finalHash := sha256.Sum256(signatureMessage)

	if !ed25519.Verify(pubKey, finalHash[:], sig) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// ParseRawAddress парсит строку вида "0:abcdef..." в workchain и address hash.
func ParseRawAddress(raw string) (workchain int32, addrHash []byte, err error) {
	if len(raw) < 3 || raw[1] != ':' {
		// Попробуем альтернативный формат "-1:abcdef..."
		var wc int
		var hashHex string
		n, _ := fmt.Sscanf(raw, "%d:%s", &wc, &hashHex)
		if n != 2 {
			return 0, nil, fmt.Errorf("invalid raw address format: %s", raw)
		}
		workchain = int32(wc)
		addrHash, err = hex.DecodeString(hashHex)
		if err != nil {
			return 0, nil, fmt.Errorf("invalid address hash hex: %w", err)
		}
		return workchain, addrHash, nil
	}

	var wc int
	var hashHex string
	n, _ := fmt.Sscanf(raw, "%d:%s", &wc, &hashHex)
	if n != 2 {
		return 0, nil, fmt.Errorf("invalid raw address format: %s", raw)
	}
	workchain = int32(wc)
	addrHash, err = hex.DecodeString(hashHex)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid address hash hex: %w", err)
	}
	if len(addrHash) != 32 {
		return 0, nil, fmt.Errorf("address hash must be 32 bytes, got %d", len(addrHash))
	}

	return workchain, addrHash, nil
}

func isDomainAllowed(domain string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // если список пуст, разрешаем всё (dev mode)
	}
	for _, d := range allowed {
		if d == domain {
			return true
		}
	}
	return false
}
