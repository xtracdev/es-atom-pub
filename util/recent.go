package main

import (
	"os"
	"fmt"
	"net/http"
	"io/ioutil"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"strings"
	"encoding/base64"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
)

//Decrypt from cryptopasta commit bc3a108a5776376aa811eea34b93383837994340
//used via the CC0 license. See https://github.com/gtank/cryptopasta
func Decrypt(ciphertext []byte, key *[32]byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("malformed ciphertext")
	}

	return gcm.Open(nil,
		ciphertext[:gcm.NonceSize()],
		ciphertext[gcm.NonceSize():],
		nil,
	)
}

func main() {
	//Read the recent notifications page  and decrypt the content for grins
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s url\n", os.Args[0])
		return
	}

	resp, err := http.Get(os.Args[1] + "/notifications/recent")
	if err != nil {
		fmt.Printf("Error on get: %s", err.Error())
		return
	}

	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error on read: %s", err.Error())
		return
	}



	if resp.StatusCode != http.StatusOK {
		fmt.Println(string(bytes))
		return
	}

	//Now split the output into two parts - the encrypted key
	//and the encrypted text

	parts := strings.Split(string(bytes),"::")
	if len(parts) != 2 {
		fmt.Println("Expected two parts, got ", len(parts))
		return
	}

	//Decode the key and the text
	keyBytes,err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		fmt.Println("Error decoding key: ", err.Error())
		return
	}

	//Get the encrypted bytes
	msgBytes, err :=  base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		fmt.Println("Error decoding payload: ", err.Error())
		return
	}

	//KMS set up
	sess := session.Must(session.NewSession())
	svc := kms.New(sess)

	//Decrypt the encrytion key
	di := &kms.DecryptInput{
		CiphertextBlob:keyBytes,
	}
	decryptedKey, err := svc.Decrypt(di)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	//Use the decrypted key to decrypt the message text
	decryptKey := [32]byte{}

	copy(decryptKey[:], decryptedKey.Plaintext[0:32])

	decypted, err := Decrypt(msgBytes, &decryptKey)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("Decrypted :\n", string(decypted))

}
