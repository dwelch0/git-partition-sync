package pkg

import (
	"fmt"
	"net/url"
	"os/exec"
)

// keys are gitlab PID (gitlab_group/project_name)
// values are commit SHA
type pidToCommit map[string]string

// returns a map of project PIDs (gitlab_group/project_name) to latest commit on specified source branch
func (u *Uploader) getLatestGitlabCommits() (pidToCommit, error) {
	latestCommits := make(pidToCommit)
	for _, sync := range u.syncs {
		pid := fmt.Sprintf("%s/%s", sync.Source.Group, sync.Source.ProjectName)
		// by default, the latest commit is returned
		commit, _, err := u.glClient.Commits.GetCommit(pid, sync.Source.Branch, nil)
		if err != nil {
			return nil, err
		}
		latestCommits[pid] = commit.ID
	}
	return latestCommits, nil
}

func (u *Uploader) cloneRepos(toUpdate []*GitSync) error {
	const CLONE_DIRECTORY = "clones"

	err := u.clean(CLONE_DIRECTORY)
	if err != nil {
		return err
	}

	for _, gs := range toUpdate {
		authURL, err := u.formatAuthURL(fmt.Sprintf("%s/%s", gs.Source.Group, gs.Source.ProjectName))
		if err != nil {
			return err
		}

		args := []string{"-c", fmt.Sprintf("git clone %s", authURL)}
		cmd := exec.Command("/bin/sh", args...)
		cmd.Dir = fmt.Sprintf("%s/%s", u.workdir, CLONE_DIRECTORY)
		err = cmd.Run()
		if err != nil {
			return err
		}

		gs.repoPath = fmt.Sprintf("%s/%s/%s", u.workdir, CLONE_DIRECTORY, gs.Source.ProjectName)
	}

	return nil
}

// returns git user-auth format of remote url
func (u *Uploader) formatAuthURL(pid string) (string, error) {
	projectURL := fmt.Sprintf("%s/%s", u.glBaseURL, pid)
	parsedURL, err := url.Parse(projectURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s:%s@%s%s.git",
		parsedURL.Scheme,
		u.glUsername,
		u.glToken,
		parsedURL.Host,
		parsedURL.Path,
	), nil
}
