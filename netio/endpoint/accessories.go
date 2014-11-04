package endpoint

import(   
    "github.com/brutella/hap/netio"
    "github.com/brutella/hap/netio/controller"
    
    "net/http"
    "io/ioutil"
    "log"
)

// Handles the /accessories endpoint and returns all accessories as JSON
//
// This endpoint is not session based and the same for all connections because
// the encryption/decryption is handled by the connection automatically.
type Accessories struct {
    http.Handler
    
    controller *controller.ContainerController
}

func NewAccessories(c *controller.ContainerController) *Accessories {
    handler := Accessories{
                controller: c,
            }
    
    return &handler
}

func (handler *Accessories) ServeHTTP(response http.ResponseWriter, request *http.Request) {
    log.Println("GET /accessories")
    response.Header().Set("Content-Type", netio.HTTPContentTypeHAPJson)
    
    res, err := handler.controller.HandleGetAccessories(request.Body)
    if err != nil {
        log.Println(err)
        response.WriteHeader(http.StatusInternalServerError)
    } else {
        bytes, _ := ioutil.ReadAll(res)
        log.Println("<-  JSON:", string(bytes))
        response.Write(bytes)
    }
}