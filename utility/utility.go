package utility

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"
	"trlaas/types"

	jsonpatch "github.com/evanphx/json-patch"
)

func ApplyJsonPatch(patch []byte) (err error) {
	log.Println("Recieved json patch", string(patch[:]))
	decode_patch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		log.Println("Json patch decode fail", err.Error())
		return
	}
	original, _ := json.Marshal(types.TpaasConfigObj)
	modified, err := decode_patch.Apply(original)
	if err != nil {
		log.Println("Json patch failed", err.Error())
		return
	}
	orig, _ := json.Marshal(types.TpaasConfigObj)
	log.Println("Json patch successfull Original Value", string(orig[:]))
	var data *types.ConfigObj
	json.Unmarshal(modified, &data)
	types.TpaasConfigObj = data
	mod, _ := json.Marshal(types.TpaasConfigObj)
	log.Println("Json Patch successfull Modified value", string(mod[:]))
	return
}
func GenerateCorrelationId(min int, max int) (id int) {
	rand.Seed(time.Now().UnixNano())
	id = rand.Intn(max-min) + min
	return
}
