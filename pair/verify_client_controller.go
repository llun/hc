package pair

import(
    "github.com/brutella/hap"
    "io"
    "fmt"
    "encoding/hex"
    "bytes"
)

type VerifyClientController struct {
    context *hap.Context
    accessory *hap.Accessory
    session *VerifySession
    username string
    LTPK []byte
    LTSK []byte
}

func NewVerifyClientController(context *hap.Context, accessory *hap.Accessory, username string) *VerifyClientController {    
    LTPK, LTSK, _ := hap.ED25519GenerateKey(username)
        
    controller := VerifyClientController{
                                    username: username,
                                    context: context,
                                    accessory: accessory,
                                    session: NewVerifySession(),
                                    LTPK: LTPK,
                                    LTSK: LTSK,
                                }
    
    return &controller
}

func (c *VerifyClientController) Handle(r io.Reader) (io.Reader, error) {
    var tlv_out *hap.TLV8Container
    var err error
    
    tlv_in, err := hap.ReadTLV8(r)
    if err != nil {
        return nil, err
    }
    
    method := tlv_in.GetByte(hap.TLVType_AuthMethod)
    
    // It is valid that method is not sent
    // If method is sent then it must be 0x00
    if method != 0x00 {
        return nil, hap.NewErrorf("Cannot handle auth method %b", method)
    }
    
    seq := tlv_in.GetByte(hap.TLVType_SequenceNumber)
    fmt.Println("->     Seq:", seq)
    
    switch seq {
    case VerifyStartRespond:
        tlv_out, err = c.handlePairVerifyRespond(tlv_in)
    case VerifyFinishRespond:        
        tlv_out, err = c.handlePairVerifyFinishRespond(tlv_in)
    default:
        return nil, hap.NewErrorf("Cannot handle sequence number %d", seq)
    }
    
    if err != nil {
        fmt.Println("[ERROR]", err)
        return nil, err
    } else if tlv_out == nil {
        fmt.Println("KEY VERIFICATION FINISHED")
        return nil, nil
    } else {
        fmt.Println("<-     Seq:", tlv_out.GetByte(hap.TLVType_SequenceNumber))
    }
    
    fmt.Println("-------------")
    
    return tlv_out.BytesBuffer(), nil
}

// Client -> Server
// - Public key `A`
func (c *VerifyClientController) InitialKeyVerifyRequest() (io.Reader) {
    tlv_out := hap.TLV8Container{}
    tlv_out.SetByte(hap.TLVType_AuthMethod, 0)
    tlv_out.SetByte(hap.TLVType_SequenceNumber, VerifyStartRequest)
    tlv_out.SetBytes(hap.TLVType_PublicKey, c.session.publicKey[:])
    
    fmt.Println("<-     A:", hex.EncodeToString(tlv_out.GetBytes(hap.TLVType_PublicKey)))
    
    return tlv_out.BytesBuffer()
}

// Server -> Client
// - B: server public key
// - encrypted message
//      - username
//      - signature: from server session public key, server name, client session public key
//
// Client -> Server
// - encrypted message
//      - username
//      - signature: from client session public key, server name, server session public key,
func (c *VerifyClientController) handlePairVerifyRespond(tlv_in *hap.TLV8Container) (*hap.TLV8Container, error) {        
    serverPublicKey := tlv_in.GetBytes(hap.TLVType_PublicKey)
    if len(serverPublicKey) != 32 {
        return nil, hap.NewErrorf("Invalid server public key size %d", len(serverPublicKey))
    }
    
    var otherPublicKey [32]byte
    copy(otherPublicKey[:], serverPublicKey)
    c.session.GenerateSharedKeyWithOtherPublicKey(otherPublicKey)
    c.session.SetupEncryptionKey([]byte("Pair-Verify-Encrypt-Salt"), []byte("Pair-Verify-Encrypt-Info"))
    
    fmt.Println("Client")
    fmt.Println("->   B:", hex.EncodeToString(serverPublicKey))
    fmt.Println("     S:", hex.EncodeToString(c.session.secretKey[:]))
    fmt.Println("Shared:", hex.EncodeToString(c.session.sharedKey[:]))
    fmt.Println("     K:", hex.EncodeToString(c.session.encryptionKey[:]))
    
    // Decrypt
    data := tlv_in.GetBytes(hap.TLVType_EncryptedData)
    message := data[:(len(data) - 16)]
    var mac [16]byte
    copy(mac[:], data[len(message):]) // 16 byte (MAC)    
    
    decrypted, err := hap.Chacha20DecryptAndPoly1305Verify(c.session.encryptionKey[:], []byte("PV-Msg02"), message, mac, nil)
    if err != nil {
        return nil, err
    }
    
    decrypted_buffer := bytes.NewBuffer(decrypted)
    tlv_decrypted, err := hap.ReadTLV8(decrypted_buffer)
    if err != nil {
        return nil, err
    }
    
    username  := tlv_decrypted.GetString(hap.TLVType_Username)
    signature := tlv_decrypted.GetBytes(hap.TLVType_Ed25519Signature)
    
    fmt.Println("    Username:", username)
    fmt.Println("   Signature:", hex.EncodeToString(signature))
    
    // Validate signature
    material := make([]byte, 0)
    material = append(material, c.session.otherPublicKey[:]...)
    material = append(material, username...)
    material = append(material, c.session.publicKey[:]...)
    
    if hap.ValidateED25519Signature(c.accessory.PublicKey, material, signature) == false {
        return nil, hap.NewErrorf("Could not validate signature")
    }
    
    tlv_out := hap.TLV8Container{}
    tlv_out.SetByte(hap.TLVType_AuthMethod, 0)
    tlv_out.SetByte(hap.TLVType_SequenceNumber, VerifyFinishRequest)
    
    tlv_encrypt := hap.TLV8Container{}
    tlv_encrypt.SetString(hap.TLVType_Username, c.username)
    
    material = make([]byte, 0)
    material = append(material, c.session.publicKey[:]...)
    material = append(material, c.username...)
    material = append(material, c.session.otherPublicKey[:]...)
    
    signature, err = hap.ED25519Signature(c.LTSK, material)
    if err != nil {
        return nil, err
    }
    
    tlv_encrypt.SetBytes(hap.TLVType_Ed25519Signature, signature)
    
    encrypted, mac, _ := hap.Chacha20EncryptAndPoly1305Seal(c.session.encryptionKey[:], []byte("PV-Msg03"), tlv_encrypt.BytesBuffer().Bytes(), nil)
    
    tlv_out.SetBytes(hap.TLVType_EncryptedData, append(encrypted, mac[:]...))
    
    return &tlv_out, nil
}

// Server -> Client
// - only error ocde (optional)
func (c *VerifyClientController) handlePairVerifyFinishRespond(tlv_in *hap.TLV8Container) (*hap.TLV8Container, error) {    
    err_code := tlv_in.GetByte(hap.TLVStatus_NoError)
    if err_code != 0x00 {
        fmt.Println("Unexpected error %d", err_code)
    }
    
    return nil, nil
}