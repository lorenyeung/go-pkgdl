package main

import (
	"bufio"
	"container/list"
	"encoding/json"
	"flag"
	"fmt"
	"go-pkgdl/auth"
	"go-pkgdl/debian"
	"go-pkgdl/docker"
	"go-pkgdl/gems"
	"go-pkgdl/generic"
	"go-pkgdl/helpers"
	"go-pkgdl/maven"
	"go-pkgdl/npm"
	"go-pkgdl/pypi"
	"go-pkgdl/rpm"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var gitCommit string
var version string

func printVersion() {
	fmt.Println("Current build version:", gitCommit, "Current Version:", version)
}

func main() {
	versionFlag := flag.Bool("v", false, "Print the current version and exit")
	flags := helpers.SetFlags()
	helpers.SetLogger(flags.LogLevelVar)

	switch {
	case *versionFlag:
		printVersion()
		return
	}

	supportedTypes := [8]string{"debian", "docker", "generic", "maven", "npm", "pypi", "rpm", "gems"}
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	configFolder := "/.lorenygo/pkgDownloader/"
	configPath := usr.HomeDir + configFolder

	log.Debug("Checking existence of download folders for:", supportedTypes)
	for i := 0; i < len(supportedTypes); i++ {
		if _, err := os.Stat(configPath + supportedTypes[i] + "Downloads/"); os.IsNotExist(err) {
			log.Info("No config folder found")
			err = os.MkdirAll(configPath+supportedTypes[i]+"Downloads/", 0700)
			helpers.Check(err, true, "Generating "+configPath+" directory", helpers.Trace())
		} else {
		}
	}
	log.Debug("Done checking existence for:", supportedTypes)
	//TODO clean up downloads dir beforehand

	masterKey := auth.VerifyMasterKey(configPath + "master.key")

	creds := auth.GetDownloadJSON(configPath+"download.json", masterKey)

	if flags.UsernameVar == "" {
		flags.UsernameVar = creds.Username
	}
	if flags.ApikeyVar == "" {
		flags.ApikeyVar = creds.Apikey
	}
	if flags.URLVar == "" {
		flags.URLVar = creds.URL
	}
	credsFilelength := 0
	credsFileHash := make(map[int][]string)
	if flags.CredsFileVar != "" {
		credsFile, err := os.Open(flags.CredsFileVar)
		if err != nil {
			log.Error("Invalid creds file:", err)
			os.Exit(0)
		}
		defer credsFile.Close()
		scanner := bufio.NewScanner(credsFile)

		for scanner.Scan() {
			credsFileCreds := strings.Split(scanner.Text(), " ")
			credsFileHash[credsFilelength] = credsFileCreds
			credsFilelength = credsFilelength + 1
		}

		flags.UsernameVar = credsFileHash[0][0]
		flags.ApikeyVar = credsFileHash[0][1]
		log.Info("Number of creds in file:", credsFilelength)
		log.Info("choose first one first:", flags.UsernameVar)
	}
	//os.Exit(0)

	if (flags.RepoVar == "") && flags.ResetVar != true && flags.ValuesVar != true {
		log.Error("Must specify -repo <Repository>")
		flag.PrintDefaults()
		os.Exit(0)
	}
	if flags.ValuesVar == true {
		log.Info("User: ", creds.Username, "\nURL: ", creds.URL, "\nDownload location: ", creds.DlLocation)
		os.Exit(0)
	}

	if flags.ResetVar == true {
		creds = auth.GenerateDownloadJSON(configPath+"download.json", true, masterKey)
		flags.UsernameVar = creds.Username
		flags.ApikeyVar = creds.Apikey
		flags.URLVar = creds.URL
	}

	if !auth.VerifyAPIKey(flags.URLVar, flags.UsernameVar, flags.ApikeyVar) {
		if creds.Username == flags.UsernameVar && creds.Apikey == flags.ApikeyVar && creds.URL == flags.URLVar {
			log.Warn("Looks like there's an issue with your credentials file. Resetting")
			auth.GenerateDownloadJSON(configPath+"download.json", true, masterKey)
			creds = auth.GetDownloadJSON(configPath+"download.json", masterKey)
			flags.UsernameVar = creds.Username
			flags.ApikeyVar = creds.Apikey
			flags.URLVar = creds.URL

		} else {
			log.Error("Looks like there's an issue with your custom credentials. Exiting")
			os.Exit(1)
		}
	}

	//update custom
	creds.Username = flags.UsernameVar
	creds.Apikey = flags.ApikeyVar
	creds.URL = flags.URLVar

	var repotype, extractedURL, pypiRegistryURL, pypiRepoSuffix = checkTypeAndRepoParams(creds, flags)
	pkgRepoDlFolder := repotype + "Downloads"

	//case switch for different package types
	workQueue := list.New()
	var extractedURLStripped = strings.TrimSuffix(extractedURL, "/")
	if !strings.HasSuffix(extractedURL, "/") {
		extractedURL = extractedURL + "/"
	}
	if flags.ForceTypeVar != "" {
		repotype = flags.ForceTypeVar
	}
	switch repotype {
	case "debian":
		go func() {
			debian.GetDebianHrefs(extractedURL+"pool/", extractedURLStripped, 1, "", workQueue)
		}()

	case "docker":
		log.Warn("Work in progress, only works against Docker Hub")
		go func() {
			log.Info("testing if it goes in here multiple times case repotype") //it does not
			docker.GetDockerImages(creds.URL, creds.Username, creds.Apikey, flags.RepoVar, extractedURL, extractedURLStripped, 1, "", workQueue, flags)
		}()

	case "generic":
		log.Warn("Work in progress")
		go func() {
			log.Debug("Extraced URL:", extractedURL, " stripped:", extractedURLStripped)
			//TODO: if url does not end in /, it messes up
			generic.GetGenericHrefs(extractedURL, extractedURLStripped, workQueue, flags.RepoVar, flags)

		}()

	case "maven":
		go func() {
			maven.GetMavenHrefs(extractedURL, extractedURLStripped, workQueue, flags)
		}()

	case "npm":
		if flags.NpmRegistryOldVar {
			log.Info("Using old method")
			npm.GetNPMList(configPath, workQueue)
		} else {
			log.Info("Using search method")
			go func() {
				npm.GetNPMListNew(creds, flags, workQueue, extractedURL)
			}()
		}

	case "pypi":
		go func() {
			pypi.GetPypiHrefs(pypiRegistryURL+"/"+pypiRepoSuffix+"/", pypiRegistryURL, extractedURLStripped, flags, workQueue)
		}()

	case "rpm":
		go func() {
			log.Info("rpm takes 10 seconds to init, please be patient")
			//buggy. looks like there is a recursive search that screws it up
			rpm.GetRpmHrefs(extractedURL, extractedURLStripped, workQueue, flags)
		}()
	case "gems":
		go func() {
			log.Info("ruby takes 10 seconds to init, please be patient")
			//buggy. looks like there is a recursive search that screws it up
			gems.GetGemsHrefs(creds, extractedURL, extractedURLStripped, workQueue, flags)
		}()

	default:
		log.Println("Unsupported package type", repotype, ". We currently support the following:", supportedTypes)
		os.Exit(0)
	}

	//disk usage check
	go func() {
		for {
			log.Debug("Running Storage summary check every ", flags.DuCheckVar, " minutes")
			auth.StorageCheck(creds, flags.StorageWarningVar, flags.StorageThresholdVar)
			time.Sleep(time.Duration(flags.DuCheckVar) * time.Minute)
		}
	}()

	//work queue
	var ch = make(chan interface{}, flags.WorkersVar+1)
	var wg sync.WaitGroup
	workQueueCount := 0
	for i := 0; i < flags.WorkersVar; i++ {
		go func(i int) {
			for {

				s, ok := <-ch
				if !ok {
					log.Info("Worker being returned to queue?", i)
					wg.Done()
				}
				log.Debug("worker ", i, " starting job")
				if workQueueCount > flags.PkgLimitVar && flags.PkgLimitVar != 0 {
					log.Info("Reached limit of ", flags.PkgLimitVar, " exiting now.")
					os.Exit(0)
				} else {
					workQueueCount++
				}

				if flags.CredsFileVar != "" {
					//pick random user and password from list
					numCreds := len(credsFileHash)
					rand.Seed(time.Now().UnixNano())
					randCredIndex := rand.Intn(numCreds)
					creds.Username = credsFileHash[randCredIndex][0]
					creds.Apikey = credsFileHash[randCredIndex][1]
				}
				switch repotype {

				case "debian":
					md := s.(debian.Metadata)
					standardDownload(creds, md.URL, md.File, configPath, pkgRepoDlFolder, flags.RepoVar)
					auth.GetRestAPI("PUT", true, creds.URL+"/api/storage/"+flags.RepoVar+"-cache"+md.URL+"?properties=deb.component="+md.Component+";deb.architecture="+md.Architecture+";deb.distribution="+md.Distribution, creds.Username, creds.Apikey, "", nil, 1)

				case "docker":
					md := s.(docker.Metadata)
					docker.DlDockerLayers(creds, md, flags.RepoVar, i, false)

				case "gems":
					md := s.(gems.Metadata)
					standardDownload(creds, md.URL, md.File, configPath, pkgRepoDlFolder, flags.RepoVar)

				case "generic":
					md := s.(generic.Metadata)
					generic.GenericDownload(creds, md, configPath, pkgRepoDlFolder, flags.RepoVar, i)
					//generic.CreateAndUploadFile(creds, md, flags, configPath, pkgRepoDlFolder, i)

				case "maven":
					md := s.(maven.Metadata)
					standardDownload(creds, md.URL, md.File, configPath, pkgRepoDlFolder, flags.RepoVar)

				case "npm":
					md := s.(npm.Metadata)
					npm.GetNPMMetadata(creds, creds.URL+"/api/npm/"+flags.RepoVar+"/", md.ID, md.Package, configPath, pkgRepoDlFolder, i, flags)

				case "pypi":
					md := s.(pypi.Metadata)
					standardDownload(creds, md.URL, md.File, configPath, pkgRepoDlFolder, flags.RepoVar)

				case "rpm":
					md := s.(rpm.Metadata)
					standardDownload(creds, md.URL, md.File, configPath, pkgRepoDlFolder, flags.RepoVar)
				}
				log.Debug("worker ", i, " finished job")
			}
		}(i)

	}

	//debug port
	go func() {
		http.ListenAndServe("0.0.0.0:8080", nil)
	}()
	for {
		var count0 = 0
		for workQueue.Len() == 0 {
			log.Info(repotype, " work queue is empty, sleeping for ", flags.WorkerSleepVar, " seconds...")
			time.Sleep(time.Duration(flags.WorkerSleepVar) * time.Second)
			count0++
			if count0 > 50 {
				log.Warn("Looks like nothing's getting put into the workqueue. You might want to enable -debug and take a look")
			}
			if workQueue.Len() > 0 {
				count0 = 0
			}
		}
		s := workQueue.Front().Value
		workQueue.Remove(workQueue.Front())
		ch <- s
	}
	close(ch)
	wg.Wait()

}

func standardDownload(creds auth.Creds, dlURL string, file string, configPath string, pkgRepoDlFolder string, repoVar string) {
	_, headStatusCode, _ := auth.GetRestAPI("HEAD", true, creds.URL+"/"+repoVar+"-cache/"+dlURL, creds.Username, creds.Apikey, "", nil, 1)
	if headStatusCode == 200 {
		log.Debug("skipping, got 200 on HEAD request for ", creds.URL+"/"+repoVar+"-cache/"+dlURL)
		return
	}

	log.Info("Downloading ", creds.URL+"/"+repoVar+dlURL)
	auth.GetRestAPI("GET", true, creds.URL+"/"+repoVar+dlURL, creds.Username, creds.Apikey, configPath+pkgRepoDlFolder+"/"+file, nil, 1)
	os.Remove(configPath + pkgRepoDlFolder + "/" + file)
}

//func standardUpload()

//Test if remote repository exists and is a remote
func checkTypeAndRepoParams(creds auth.Creds, flags helpers.Flags) (string, string, string, string) {
	repoCheckData, repoStatusCode, _ := auth.GetRestAPI("GET", true, creds.URL+"/api/repositories/"+flags.RepoVar, creds.Username, creds.Apikey, "", nil, 1)
	if repoStatusCode != 200 {
		log.Error("Repo", flags.RepoVar, "does not exist.")
		os.Exit(0)
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(repoCheckData), &result)
	//TODO: hard code for now, mass upload of files
	if result["rclass"] == "local" && result["packageType"].(string) == "generic" {
		return result["packageType"].(string), "", "", ""
	} else if result["rclass"] != "remote" {
		log.Error(flags.RepoVar, "is a", result["rclass"], "repository and not a remote repository.")
		//maybe here.
		os.Exit(0)
	}
	if result["packageType"].(string) == "pypi" {
		if result["pyPIRegistryUrl"] == nil || result["pyPIRepositorySuffix"] == nil {
			log.Warn("pypi repo setting pyPIRegistryUrl/pyPIRepositorySuffix is nil, likely running older version.")
			if flags.PypiRegistryURLVar == "" || flags.PypiRepoSuffixVar == "" {
				log.Error("please manually set -pypiregistryurl and -pypireposuffix")
				os.Exit(0)
			} else {
				return result["packageType"].(string), result["url"].(string), flags.PypiRegistryURLVar, flags.PypiRepoSuffixVar
			}
		}
		return result["packageType"].(string), result["url"].(string), result["pyPIRegistryUrl"].(string), result["pyPIRepositorySuffix"].(string)
	}
	return result["packageType"].(string), result["url"].(string), "", ""
}
