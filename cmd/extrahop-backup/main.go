package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"bosun.org/slog"

	"github.com/mhenderson-so/extrahop-backup/version"
)

var (
	fEHHost   = flag.String("host", "", "URL to the ExtraHop host. E.g. https://extrahop01.example.com.")
	fEHAPIKey = flag.String("apikey", "", "Your API Key for the ExtraHop host.")
	fGitDir   = flag.String("gitdir", os.TempDir(), "Optional. Directory to do a git clone into.")
	fGitRepo  = flag.String("gitrepo", "", "Git repository to store backups into.")
	fPrintV   = flag.Bool("v", false, "Alias to -version.")
	fPrintVer = flag.Bool("version", false, "Print version information.")
	fVerbose  = flag.Bool("verbose", false, "Output verbose details.")
)

func main() {
	flag.Parse()

	if *fPrintVer || *fPrintV {
		fmt.Println(version.GetVersionInfo("extrahop-backup"))
		return
	}

	if *fEHHost == "" {
		fmt.Fprintln(os.Stderr, "ExtraHop -host has not been specified")
		return
	}

	if *fEHAPIKey == "" {
		fmt.Fprintln(os.Stderr, "ExtraHop -apikey has not been specified")
		return
	}

	if *fGitRepo == "" {
		fmt.Fprintln(os.Stderr, "Git -gitrepo has not been specified")
		return
	}

	//Create the temporary folder that the backup git repo will be checked out into. This will be deleted at
	//the end. A fresh checkout will be done every time.
	if *fGitDir == os.TempDir() {
		*fGitDir, _ = ioutil.TempDir("", "ExtraHopBackup")
		if *fVerbose {
			slog.Infoln("Setting temporary git checkout directory to", *fGitDir)
		}
	} else {
		*fGitDir += "ExtraHopBackup"
		os.MkdirAll(*fGitDir, 0755)
	}

	//Clone the repo into our output directory
	gitClone(*fGitDir, *fGitRepo)

	//Create a client, so that we can reuse this client connection multiple times for requests
	client := &http.Client{}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		req.Header = via[0].Header
		return nil
	}

	// TODO (mhenderson) 2015-05-04: This could be sped up by throwing these into goroutines(). We have a common
	// http client that they can use to share the same connection. In fact, we could be fetching these
	// even while we are checking out the git repo.

	// TODO (mhenderson) 2015-05-04: Make this modular so you can specify which endpoints to backup from the command
	// line, or a config file, or something along those lines.

	//Grab the endpoints and put them into the git repo
	backupEHEndpoint("runningconfig", client)
	backupEHEndpoint("triggers", client)
	backupEHEndpoint("auditlog?limit=999999&offset=0", client)

	//Add every file in the git repo and push
	pushAll(*fGitDir, "master")

	//Delete the temporary folder
	if *fVerbose {
		slog.Infoln("Cleaning up. Deleting ", *fGitDir)
	}

	dirErr := os.RemoveAll(*fGitDir)

	//Git for Windows goes and puts some read-only files in the directory, which Go can't delete by default.
	//So if we run into this, we can do an OS call to fix that right up.
	if dirErr != nil && runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/C", "rmdir", "/S", "/Q", *fGitDir)
		if *fVerbose {
			slog.Warningln("Cleaning up. Can't delete:", dirErr)
			slog.Warningln("Cleaning up. Invoking OS deletion:", cmd.Args)
		}
		cmd.Run()
	}
}

// backupEHEndpoint takes an ExtraHop API endpoint and dumps it into the Git repository
func backupEHEndpoint(endpoint string, client *http.Client) error {
	ehBody, err := invokeEHAPI(endpoint, client)
	if err != nil {
		return err
	}

	endpointName := strings.Split(endpoint, "?")[0]
	backupFile := path.Join(*fGitDir, fmt.Sprintf("%v.json", endpointName))

	if *fVerbose {
		slog.Infoln("Writing", endpoint, "to", backupFile)
	}

	err = ioutil.WriteFile(backupFile, ehBody, 0755)
	return err
}

// invokeEHAPI takes an ExtraHop API endpoint and returns some pretty-printed JSON of the returned
// values.
func invokeEHAPI(endpoint string, client *http.Client) ([]byte, error) {
	ehURL, err := url.Parse(fmt.Sprintf("%v/api/v1/%v", *fEHHost, endpoint))
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", ehURL.String(), nil)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "extrahop-backupclient(github.com/mhenderson-so/extrahop-backup)")
	req.Header.Set("Authorization", fmt.Sprintf("ExtraHop apikey=%v", *fEHAPIKey))

	if *fVerbose {
		slog.Infoln("Requesting", req.URL.String())
	}
	res, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// http://stackoverflow.com/a/29046984/69683
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, body, "", "\t")

	return prettyJSON.Bytes(), err
}

// setVerbose sets the output of exec.cmd processes to stdout
func setVerbose(cmd *exec.Cmd) {
	if *fVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
}

// gitClone does what it says on the tin. It clones a git repo to a directory.
func gitClone(tmpDir, repoDir string) {
	// clone in a temporary rep
	for tries := 0; ; tries++ {
		// depth: speed up things
		cmd := exec.Command("git", "clone", *fGitRepo, tmpDir,
			"--depth=1", "--no-single-branch")
		setVerbose(cmd)
		if err := cmd.Run(); err == nil {
			return
		} else if tries >= 2 {
			slog.Errorf("could not clone the repo: %v, %v", cmd, err)
			panic("could not clone the repo")
		}
	}
}

// pushAll adds all the files in a repo, adds a default commit message, and pushes it.
func pushAll(repoDir, branch string) {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	setVerbose(cmd)
	if err := cmd.Run(); err != nil {
		slog.Errorf("could not add into git: %v", cmd)
		panic("could not add into git")
	}

	msg := "autoCommit"
	// get Added and Changed files TODO
	// git ls-files --others --exclude-standar : added
	// git ls-files -m : changed

	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = repoDir
	txt, err := cmd.CombinedOutput()
	if strings.Contains(string(txt), "nothing to commit") {
		if *fVerbose {
			slog.Infoln("No changes detected, don't push")
		}
		return
	}
	if err != nil {
		slog.Errorln("could not read git commit output:", err)
		panic("could not read git commit output")
	}

	if *fVerbose {
		fmt.Printf(string(txt))
	}
	cmd = exec.Command("git", "push", "--set-upstream", "origin", branch)
	cmd.Dir = repoDir
	setVerbose(cmd)
	if err = cmd.Run(); err != nil {
		slog.Errorf("could not push: %v", cmd)
		panic("could not push")
	}
}
