package npm

import (
	"container/list"
	"encoding/json"
	"go-pkgdl/auth"
	"go-pkgdl/helpers"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

//Metadata blah
type artifactMetadata struct {
	Versions map[string]distMetadata
}

//DistMetadata blah
type distMetadata struct {
	Dist struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

//Metadata for worker queue
type Metadata struct {
	Rows []struct {
		ID string `json:"id"`
	} `json:"rows"`
	ID      string
	Package string
}

//GetNPMMetadata blah
func GetNPMMetadata(creds auth.Creds, URL, packageIndex, packageName, configPath string, dlFolder string, workerNum int, flags helpers.Flags) {
	data, _, _ := auth.GetRestAPI("GET", true, URL+packageName, creds.Username, creds.Apikey, "", nil, 1)
	var metadata = artifactMetadata{}
	err := json.Unmarshal([]byte(data), &metadata)
	if err != nil {
		log.Error("Worker ", workerNum, " error:"+err.Error())
	}
	for i, j := range metadata.Versions {

		s := strings.Split(j.Dist.Tarball, "api/npm/"+flags.RepoVar)
		//fmt.Println(len(s), "length of s") //413 error
		if len(s) > 1 && s[1] != "" {
			_, headStatusCode, _ := auth.GetRestAPI("HEAD", true, creds.URL+"/"+flags.RepoVar+"-cache/"+s[1], creds.Username, creds.Apikey, "", nil, 1)
			if headStatusCode == 200 {
				log.Debug("Worker ", workerNum, " skipping, got 200 on HEAD request for ", creds.URL+"/"+flags.RepoVar+"-cache/"+s[1])
				continue
			}
		}
		if !flags.NpmMetadataVar {
			packageDl := packageIndex + "-" + i + ".tgz"
			log.Info("Worker ", workerNum, " Downloading ", s[1])
			auth.GetRestAPI("GET", true, j.Dist.Tarball, creds.Username, creds.Apikey, configPath+dlFolder+"/"+packageDl, nil, 1)
			err2 := os.Remove(configPath + dlFolder + "/" + packageDl)
			helpers.Check(err2, false, "Deleting file", helpers.Trace())
		}
	}
	helpers.Check(err, false, "Reading", helpers.Trace())
	if err != nil {
		return
	}
}

//GetNPMList function to convert raw list into readable text file
func GetNPMList(configPath string, npmWorkQueue *list.List) {
	if _, err := os.Stat(configPath + "all-npm.json"); os.IsNotExist(err) {
		log.Info("No all-npm.json found, creating...")
		auth.GetRestAPI("GET", false, "https://replicate.npmjs.com/_all_docs", "", "", configPath+"all-npm.json", nil, 1)
	}
	var result Metadata
	file, err := os.Open(configPath + "all-npm.json")
	helpers.Check(err, true, "npm JSON read", helpers.Trace())
	byteValue, _ := ioutil.ReadAll(file)
	json.Unmarshal([]byte(byteValue), &result)
	for i, j := range result.Rows {
		t := strconv.Itoa(i)
		result.ID = t
		result.Package = j.ID
		npmWorkQueue.PushBack(result)
	}
}
