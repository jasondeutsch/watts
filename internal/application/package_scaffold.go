package application

import "github.com/jasondeutsch/watts/internal/workflowlang"

func (app *Service) NewPackage(destination, source string, options workflowlang.Options) (string, error) {
	return workflowlang.Scaffold(destination, source, options)
}
