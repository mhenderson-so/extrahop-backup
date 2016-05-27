// Simple script to build extrahop-backup client and server. This is not required, but it will properly insert version date and commit
// metadata into the resulting binaries, which `go build` will not do by default.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/josephspurrier/goversioninfo"
	"github.com/mhenderson-so/extrahop-backup/version"
)

var (
	shaFlag         = flag.String("sha", "", "SHA to embed. Omit to pull from current repository")
	buildDriverTest = flag.Bool("drivers", false, "Only build Driver Test (not included in normal build).")
	buildOS         = flag.String("os", runtime.GOOS, "OS to build for.")
	buildOfficial   = flag.Bool("release", false, "Release build. Used in -version info only.")
	buildBranch     = flag.String("branch", "", "Branch name. Used in -version info only. Omit to pull from current repository.")
	output          = flag.String("output", os.Getenv("GOBIN"), "Output directory")
	allProgs        = []buildTarget{buildTarget{"extrahop-backup", "extrahop-backup"}}
)

type buildTarget struct {
	dirName    string
	binaryName string
}

func main() {
	flag.Parse()

	*buildOS = strings.ToLower(*buildOS)
	origBuildOS := os.Getenv("GOOS")
	os.Setenv("GOOS", *buildOS)

	goPath := os.Getenv("GOPATH")

	// Get current commit SHA
	sha := *shaFlag
	if sha == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Stderr = os.Stderr
		output, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		sha = strings.TrimSpace(string(output))
	}

	//Get current branch name
	branchName := *buildBranch
	if branchName == "" {
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Stderr = os.Stderr
		output, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		branchName = strings.TrimSpace(string(output))
	}

	var buildRelease string
	if *buildOfficial || branchName == "master" {
		buildRelease = "true"
	}

	timeStr := time.Now().UTC().Format("20060102150405")
	ldFlags := fmt.Sprintf("-X github.com/mhenderson-so/extrahop-backup/version.VersionSHA=%s -X github.com/mhenderson-so/extrahop-backup/version.VersionDate=%s -X github.com/mhenderson-so/extrahop-backup/version.OfficialBuild=%s -X github.com/mhenderson-so/extrahop-backup/version.BuildBranch=%s", sha, timeStr, buildRelease, branchName)

	var err error

	for _, app := range allProgs {
		var buildArgs, genArgs []string

		if *buildOS == "windows" {
			app.binaryName += ".exe"

			//Path to the config for goversioninfo
			versionInfoFile := filepath.Join(goPath, "src", "github.com", "mhenderson-so", "extrahop-backup", "cmd", app.dirName, "versioninfo.json")
			//Contents of the config
			versionInfoBytes, err := ioutil.ReadFile(fmt.Sprintf(versionInfoFile))

			if err == nil { //If we have a goversioninfo config file
				versionNos := strings.Split(version.BuildVersion.Version, ".") //Get the version numbers
				versionMajor, _ := strconv.Atoi(versionNos[0])                 //Major as string
				versionMinor, _ := strconv.Atoi(versionNos[1])                 //Minor as string

				//Set these into the version pkg so that we have the same data here as we will in the compiled program
				version.BuildVersion.OfficialBuild = buildRelease
				version.BuildVersion.VersionDate = timeStr
				version.BuildVersion.VersionSHA = sha

				//This will hold the contents of versioninfo.json
				versionInfo := &goversioninfo.VersionInfo{}
				err = versionInfo.ParseJSON(versionInfoBytes) //Load the JSON into the struct

				//If we have a version number and a populated VersionInfo{}
				if err == nil && (versionMajor > 0 || versionMinor > 0) {
					//Set all the properties that we want to set for the binary
					versionInfo.Build()
					versionInfo.FixedFileInfo.FileVersion.Major = versionMajor
					versionInfo.FixedFileInfo.FileVersion.Minor = versionMinor
					versionInfo.FixedFileInfo.ProductVersion.Major = versionMajor
					versionInfo.FixedFileInfo.ProductVersion.Minor = versionMinor
					versionInfo.StringFileInfo.ProductVersion = sha
					versionInfo.StringFileInfo.Comments = version.GetVersionInfo(app.binaryName)
					versionInfo.StringFileInfo.OriginalFilename = app.binaryName
				}
				//We need to write the config JSON back to the original file so that it can be picked up by the linker
				versionInfoBytes, err := json.Marshal(versionInfo)
				if err == nil {
					ioutil.WriteFile(versionInfoFile, versionInfoBytes, 0755)
					//Add parameters to the "go generate" array

				}
			}
		}

		buildArgs = append(buildArgs, "build", "-o", filepath.Join(*output, app.binaryName))
		buildArgs = append(buildArgs, "-ldflags", ldFlags, fmt.Sprintf("github.com/mhenderson-so/extrahop-backup/cmd/%s", app.dirName))
		genArgs = append(genArgs, "generate", fmt.Sprintf("github.com/mhenderson-so/extrahop-backup/cmd/%s", app.dirName))

		//We always want to do a go generate. No harm in doing this anyway.
		//Check that we have goversioninfo in the path
		goVI, _ := exec.Command("goversioninfo", "-?").CombinedOutput() //This should get the help output
		if len(goVI) == 0 {                                             //If we don't have any help
			fmt.Println("[missing goversioninfo, attempting to install]")
			outInstall, _ := exec.Command("go", "install", "github.com/mhenderson-so/extrahop-backup/vendor/github.com/josephspurrier/goversioninfo/cmd/goversioninfo").CombinedOutput()
			fmt.Println(string(outInstall)) //Install it
		}

		fmt.Println(genArgs)                     //Should output like "generate gitlab.stackexhange... etc"
		genCmd := exec.Command("go", genArgs...) //Create a Go Generate command
		genCmd.Stdout = os.Stdout                //Pipe everything to the normal std. outputs
		genCmd.Stderr = os.Stderr
		genCmd.Run() //Generate

		fmt.Println("building", filepath.Join(*output, app.binaryName), "for", *buildOS)
		cmd := exec.Command("go", buildArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	os.Setenv("GOOS", origBuildOS)
}
