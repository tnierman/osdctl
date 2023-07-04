package promote

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openshift/osdctl/cmd/promote/git"
)

const (
	OSDSaasDir = "data/services/osd-operators/cicd/saas"
	BPSaasDir  = "data/services/backplane/cicd/saas"
	CADSaasDir = "data/services/configuration-anomaly-detection/cicd"
)

func GetServiceNames(saaDirs ...string) ([]string, error) {
	services := []string{}
	baseDir, err := git.GetBaseDir()
	if err != nil {
		return services, err
	}
	for _, dir := range saaDirs {
		dirGlob := filepath.Join(baseDir, dir, "saas-*")
		filepaths, err := filepath.Glob(dirGlob)
		if err != nil {
			return nil, err
		}
		for _, filepath := range filepaths {
			// TODO: replace w/ path/filepath builtins
			filename := strings.TrimPrefix(filepath, baseDir+"/"+dir+"/")
			filename = strings.TrimSuffix(filename, ".yaml")
			services = append(services, filename)
		}
	}

	return services, nil
}

func GetServiceFiles(saasDirs ...string) (map[string]string, error){
	files := map[string]string{}
	traverseServiceFiles(saasDirs, func(path string) error {
		filename := filepath.Base(path)
		filename = strings.TrimSuffix(filename, filepath.Ext(filename))
		files[filename] = path
		return nil
	})
	return files, nil
}


func traverseServiceFiles(saasDirs []string, f func(filepath string) (error)) error {
	baseDir, err := git.GetBaseDir()
	if err != nil {
		return err
	}
	for _, dir := range saasDirs {
		dirGlob := filepath.Join(baseDir, dir, "saas-*")
		filepaths, err := filepath.Glob(dirGlob)
		if err != nil {
			return err
		}
		for _, filepath := range filepaths {
			err = f(filepath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateServiceName(services []string, serviceName string) error {
	fmt.Printf("### Checking if service %s exists ###\n", serviceName)
	for _, service := range services {
		if service == serviceName {
			fmt.Printf("Service %s found\n", serviceName)
			return nil
		}
	}

	return fmt.Errorf("service %s not found", serviceName)
}

func getSaasDir(serviceName string, osd bool, hcp bool) (string, error) {
	serviceFiles, err := GetServiceFiles()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve service files: %w", err)
	}
	if saasDir, ok := serviceFiles[serviceName]; ok {
		if strings.Contains(saasDir, ".yaml") && osd {
			return saasDir, nil
		}

		// This is a hack while we migrate the rest of the operators unto Progressive Delivery
		if osd {
			saasDir = saasDir + "/deploy.yaml"
			return saasDir, nil
		} else if hcp {
			saasDir = saasDir + "/hypershift-deploy.yaml"
			return saasDir, nil
		}
	}

	return "", fmt.Errorf("saas directory for service %s not found", serviceName)
}
