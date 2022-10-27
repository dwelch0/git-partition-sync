package pkg

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/xanzy/go-gitlab"

	"gopkg.in/yaml.v3"
)

type GitTarget struct {
	Group       string `yaml:"group"`
	ProjectName string `yaml:"project_name"`
	Branch      string `yaml:"branch"`
}

type GitSync struct {
	Source      GitTarget `yaml:"source"`
	Destination GitTarget `yaml:"destination"`
	repoPath    string
	tarPath     string
	encryptPath string
}

type Uploader struct {
	awsRegion  string
	bucket     string
	glBaseURL  string
	glUsername string
	glToken    string
	publicKey  string
	workdir    string

	glClient *gitlab.Client
	s3Client *s3.Client

	syncs []*GitSync
}

func NewUploader(rawCfg []byte,
	awsAccessKey,
	awsSecretKey,
	awsRegion,
	bucket,
	glURL,
	glUsername,
	glToken,
	pubKey,
	workdir string) (*Uploader, error) {

	var cfg []*GitSync
	err := yaml.Unmarshal(rawCfg, &cfg)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("mkdir", "-p", workdir)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	gl, err := gitlab.NewClient(
		glToken, gitlab.WithBaseURL(fmt.Sprintf("%s/api/v4", glURL)))
	if err != nil {
		return nil, err
	}

	awsS3 := s3.New(s3.Options{
		Region: awsRegion,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			awsAccessKey,
			awsSecretKey,
			"",
		)),
	})

	return &Uploader{
		awsRegion:  awsRegion,
		bucket:     bucket,
		glBaseURL:  glURL,
		glUsername: glUsername,
		glToken:    glToken,
		publicKey:  pubKey,
		workdir:    workdir,
		glClient:   gl,
		s3Client:   awsS3,
		syncs:      cfg,
	}, nil
}

// Run executes steps to reconcile s3 bucket with existing state of gitlab projects
func (u *Uploader) Run(ctx context.Context, dryRun bool) error {
	log.Println("Starting run...")

	glCommits, err := u.getLatestGitlabCommits()
	if err != nil {
		return err
	}

	s3ObjectInfos, err := u.getS3Keys(ctx)
	if err != nil {
		return err
	}

	toUpdate, toDelete, err := u.getOutOfSync(ctx, glCommits, s3ObjectInfos)
	if err != nil {
		return err
	}

	err = u.cloneRepos(toUpdate)
	if err != nil {
		return err
	}

	err = u.tarRepos(toUpdate)
	if err != nil {
		return err
	}

	err = u.encryptRepoTars(toUpdate)
	if err != nil {
		return err
	}

	if dryRun {
		printDryRun(toUpdate, toDelete)
		return nil
	}

	err = u.uploadLatest(ctx, toUpdate, glCommits)
	if err != nil {
		return err
	}
	for _, update := range toUpdate {
		fmt.Println(fmt.Sprintf("s3 object for destination PID `%s/%s` successfully updated",
			update.Destination.Group,
			update.Destination.ProjectName))
	}

	err = u.removeOutdated(ctx, toDelete)
	if err != nil {
		return err
	}
	for _, delete := range toDelete {
		fmt.Println(fmt.Sprintf("s3 object with key `%s` successfully deleted", *delete))
	}

	log.Println("Run successfully completed")
	return nil
}

type DecodedKey struct {
	Group       string `json:"group"`
	ProjectName string `json:"project_name"`
	CommitSHA   string `json:"commit_sha"`
	Branch      string `json:"branch"`
}

// iterates through desired Syncs (defined within config file)
// and compares latest commits on source GitLab projects against
// commits stored within s3 keys for corresponding destination GitLab projects
// return is slice of Sync that do not exist within s3Commits OR s3Commit != glCommit
// and slice of s3 object keys to delete
func (u *Uploader) getOutOfSync(ctx context.Context, glCommits pidToCommit,
	objInfos map[string]*s3ObjectInfo) ([]*GitSync, []*string, error) {

	outdated := []*GitSync{}
	toDelete := []*string{}
	for _, sync := range u.syncs {
		sourcePid := fmt.Sprintf("%s/%s", sync.Source.Group, sync.Source.ProjectName)
		destinationPid := fmt.Sprintf("%s/%s", sync.Destination.Group, sync.Destination.ProjectName)

		objInfo, exist := objInfos[destinationPid]
		if !exist {
			// new target added to config file
			outdated = append(outdated, sync)
		} else if objInfo.CommitSHA != glCommits[sourcePid] {
			// existing target is out of date
			outdated = append(outdated, sync)
			toDelete = append(toDelete, objInfo.Key)

			delete(objInfos, destinationPid) // remove processed keys from s3 bucket map
		} else {
			// target is up to date
			delete(objInfos, destinationPid)
		}
	}

	// if map is not empty at end, there are s3 keys that should be deleted
	// i.e removed from config file as targets
	for _, obj := range objInfos {
		toDelete = append(toDelete, obj.Key)
	}

	return outdated, toDelete, nil
}

// clean target working directory
func (u *Uploader) clean(directory string) error {
	cmd := exec.Command("rm", "-rf", directory)
	cmd.Dir = u.workdir
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("mkdir", directory)
	cmd.Dir = u.workdir
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func printDryRun(toUpdate []*GitSync, toDelete []*string) {
	for _, update := range toUpdate {
		fmt.Println(fmt.Sprintf("[DRY RUN] s3 object for destination PID `%s/%s` will be updated",
			update.Destination.Group,
			update.Destination.ProjectName))
	}
	for _, delete := range toDelete {
		fmt.Println(fmt.Sprintf("[DRY RUN] s3 object with key `%s` will be deleted", *delete))
	}
	log.Println("[DRY RUN] Run successfully completed")
}
