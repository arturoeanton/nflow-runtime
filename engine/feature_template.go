package engine

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

var (
	templateRepo *RepositoryTemplate
	muTemplate   sync.Mutex
)

type RepositoryTemplate struct {
	templates map[string]string
	mu        sync.Mutex
	dinamic   bool
}

func GetRepositoryTemplate() *RepositoryTemplate {
	muTemplate.Lock()
	defer muTemplate.Unlock()
	if templateRepo != nil {
		return templateRepo
	}
	templateRepo = &RepositoryTemplate{
		templates: make(map[string]string),
		dinamic:   false,
		mu:        sync.Mutex{},
	}
	return templateRepo
}

func (r *RepositoryTemplate) IsDinamic() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dinamic
}

func (r *RepositoryTemplate) SetDinamic(dinamic bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dinamic = dinamic
}

func (r *RepositoryTemplate) InicializarTemplate(templates map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates = templates
}

func (r *RepositoryTemplate) AddTemplate(name, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[name] = content
}

func (r *RepositoryTemplate) GetTemplate(name string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.dinamic { // If not dynamic, check the in-memory map first
		content, exists := r.templates[name]
		if exists {
			return content, true
		}
	}

	templatePath := os.Getenv("NFLOW_TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "./templates"
	}

	filename := templatePath + "/" + name + ".html"
	content, exist := checkAndGetFileTemplate(filename)
	if exist {
		r.templates[name] = content
		return content, true
	}

	filename = templatePath + "/" + name + ".tmpl"
	content, exist = checkAndGetFileTemplate(filename)
	if exist {
		r.templates[name] = content
		return content, true
	}

	filename = templatePath + "/" + name
	content, exist = checkAndGetFileTemplate(filename)
	if exist {
		r.templates[name] = content
		return content, true
	}

	// If not found in the map or file, check the database
	t, exist := getTemplateFromDB(name)
	if exist {
		r.templates[name] = t
		return t, true
	}

	return "", false

}

func (r *RepositoryTemplate) RemoveTemplate(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.templates, name)
}
func (r *RepositoryTemplate) ListTemplates() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}
func (r *RepositoryTemplate) ClearTemplates() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates = make(map[string]string)
}
func (r *RepositoryTemplate) TemplateCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.templates)
}
func (r *RepositoryTemplate) HasTemplate(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.templates[name]
	return exists
}

func getTemplateFromDB(paramName string) (string, bool) {
	db, err := GetDB()
	if err != nil {
		log.Println(err)
		return "", false
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		log.Println(err)
		return "", false
	}
	defer conn.Close()
	config := GetConfig()
	row := conn.QueryRowContext(context.Background(), config.DatabaseNflow.QueryGetTemplate, paramName)

	var id int
	var name string
	var content string

	err = row.Scan(&id, &name, &content)
	if err != nil {
		log.Println(err)
		return "", false
	}

	return content, true
}

func AddFeatureTemplate(vm *goja.Runtime, c echo.Context) {

	vm.Set("get_template", func(paramName string) string {

		t, exist := templateRepo.GetTemplate(paramName)
		if !exist {
			logger.Error("Template not found:", paramName)
			return ""
		}
		return t

	})

}

func checkAndGetFileTemplate(filename string) (string, bool) {
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		content, err := os.ReadFile(filename)
		if err != nil {
			logger.Error("Failed to read template file:", err)
			return "", true
		}
		return string(content), true
	}
	return "", false
}
