package special

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/control"
)

func ReceiveBodyControllerState(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	vin := params["vin"]

	control.BodyControllerState(vin)
}
