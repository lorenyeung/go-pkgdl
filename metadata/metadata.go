package metadata

import (
	"encoding/json"
	"fmt"
	"go-npmdl/auth"
	"go-npmdl/helpers"
	"log"
	"net/http"
	"os"
)

//Metadata blah
type Metadata struct {
	Versions map[string]DistMetadata
}

//DistMetadata blah
type DistMetadata struct {
	Dist struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

//GetNPMMetadata blah
func GetNPMMetadata(creds auth.Creds, URL, packageIndex, packageName, configPath string) {
	//log.Printf("Getting metadata for %s%s", URL, packageName)

	//TODO do a head request to skip ahead if it already exists in artifactory
	data := auth.GetRestAPI(true, URL+packageName, creds.Username, creds.Apikey, "")

	var metadata = Metadata{}
	err := json.Unmarshal([]byte(data), &metadata)
	if err != nil {
		fmt.Println("error:" + err.Error())
	}
	for i, j := range metadata.Versions {
		packageDl := packageIndex + "-" + i + ".tgz"

		res, err := http.Head(j.Dist.Tarball)
		if err != nil {
			panic(err)
		}
		if res.StatusCode == 200 {
			log.Printf("skipping %s, got 200 on HEAD request\n", j.Dist.Tarball)
			continue
		}
		log.Println(packageIndex, i, j.Dist.Tarball, configPath+"downloads/"+packageDl)
		auth.GetRestAPI(true, j.Dist.Tarball, creds.Username, creds.Apikey, configPath+"downloads/"+packageDl)
		err2 := os.Remove(configPath + "downloads/" + packageDl)
		helpers.Check(err2, false, "Deleting file")
	}
	helpers.Check(err, false, "Reading")
	if err != nil {
		return
	}
}
