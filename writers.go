package gorma

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"bitbucket.org/pkg/inflect"

	"github.com/goadesign/goa/design"
	"github.com/goadesign/goa/goagen/codegen"
	"github.com/kr/pretty"
)

type (
	// UserTypeTemplateData contains all the information used by the template to redner the
	// media types code.
	UserTypeTemplateData struct {
		APIDefinition *design.APIDefinition
		UserType      *RelationalModelDefinition
		DefaultPkg    string
		AppPkg        string
	}
	// UserTypesWriter generate code for a goa application user types.
	// User types are data structures defined in the DSL with "Type".
	UserTypesWriter struct {
		*codegen.SourceFile
		UserTypeTmpl *template.Template
	}
)

func fieldAssignmentModelToType(model *RelationalModelDefinition, ut *design.ViewDefinition, mtype, utype string) string {
	//utPackage := "app"
	var fieldAssignments []string
	// type.Field = model.Field
	for fname, field := range model.RelationalFields {
		if field.Datatype == "" {
			continue
		}
		var mpointer, upointer bool
		mpointer = field.Nullable
		obj := ut.Type.ToObject()
		definition := ut.Parent.Definition()
		for key := range obj {
			gfield := obj[key]
			if field.Underscore() == key || field.DatabaseFieldName == key {
				// this is our field
				if gfield.Type.IsObject() || definition.IsPrimitivePointer(key) {
					upointer = true
				} else {
					// set it explicity because we're reusing the same bool
					upointer = false
				}

				prefix := "&"
				if upointer && !mpointer {
					// ufield = &mfield
					prefix = "&"
				} else if mpointer && !upointer {
					// ufield = *mfield (rare if never?)
					prefix = ""
				} else if !upointer && !mpointer {
					prefix = ""
				}

				fa := fmt.Sprintf("\t%s.%s = %s%s", utype, codegen.Goify(key, true), prefix, codegen.Goify(fname, false))
				fieldAssignments = append(fieldAssignments, fa)
			}
		}
	}
	return strings.Join(fieldAssignments, "\n")
}

func fieldAssignmentTypeToModel(model *RelationalModelDefinition, ut *design.UserTypeDefinition, utype, mtype string) string {
	//utPackage := "app"
	var fieldAssignments []string
	// type.Field = model.Field
	for fname, field := range model.RelationalFields {
		var mpointer, upointer bool
		mpointer = field.Nullable
		obj := ut.ToObject()
		definition := ut.Definition()
		if field.Datatype == "" {
			continue
		}
		for key := range obj {
			gfield := obj[key]
			if field.Underscore() == key || field.DatabaseFieldName == key {
				// this is our field
				if gfield.Type.IsObject() || definition.IsPrimitivePointer(key) {
					upointer = true
				} else {
					// set it explicity because we're reusing the same bool
					upointer = false
				}

				var prefix string
				if upointer != mpointer {
					prefix = "*"
				}

				fa := fmt.Sprintf("\t%s.%s = %s%s.%s", mtype, fname, prefix, utype, codegen.Goify(key, true))
				fieldAssignments = append(fieldAssignments, fa)
			}
		}

	}
	return strings.Join(fieldAssignments, "\n")
}

func viewSelect(ut *RelationalModelDefinition, v *design.ViewDefinition) string {
	obj := v.Type.(design.Object)
	var fields []string
	for name := range obj {
		if obj[name].Type.IsPrimitive() {
			if strings.TrimSpace(name) != "" && name != "links" {
				bf, ok := ut.RelationalFields[codegen.Goify(name, true)]
				if ok {
					if bf.Alias != "" {
						fields = append(fields, bf.Alias)
					} else {
						fields = append(fields, bf.DatabaseFieldName)
					}
				}
			}
		}
	}
	sort.Strings(fields)
	return strings.Join(fields, ",")
}
func viewFields(ut *RelationalModelDefinition, v *design.ViewDefinition) []*RelationalFieldDefinition {
	obj := v.Type.(design.Object)
	var fields []*RelationalFieldDefinition
	for name := range obj {
		if obj[name].Type.IsPrimitive() {
			if strings.TrimSpace(name) != "" && name != "links" {
				bf, ok := ut.RelationalFields[codegen.Goify(name, true)]
				if ok {
					fields = append(fields, bf)
				}
			} else if name == "links" {
				for n, ld := range v.Parent.Links {
					fmt.Println(n)
					pretty.Println(ld.Name, ld.View)
				}
			}
		}
	}

	return fields
}

func viewFieldNames(ut *RelationalModelDefinition, v *design.ViewDefinition) []string {
	obj := v.Type.(design.Object)
	var fields []string
	for name := range obj {
		if obj[name].Type.IsPrimitive() {
			if strings.TrimSpace(name) != "" && name != "links" {
				bf, ok := ut.RelationalFields[codegen.Goify(name, true)]

				if ok {
					fields = append(fields, "&"+codegen.Goify(bf.Name, false))
				}
			}
		}
	}

	sort.Strings(fields)
	return fields
}

// NewUserTypesWriter returns a contexts code writer.
// User types contain custom data structured defined in the DSL with "Type".
func NewUserTypesWriter(filename string) (*UserTypesWriter, error) {
	file, err := codegen.SourceFileFor(filename)
	if err != nil {
		return nil, err
	}
	return &UserTypesWriter{SourceFile: file}, nil
}

// Execute writes the code for the context types to the writer.
func (w *UserTypesWriter) Execute(data *UserTypeTemplateData) error {
	fm := make(map[string]interface{})
	fm["famt"] = fieldAssignmentModelToType
	fm["fatm"] = fieldAssignmentTypeToModel
	fm["viewSelect"] = viewSelect
	fm["viewFields"] = viewFields
	fm["viewFieldNames"] = viewFieldNames
	fm["goDatatype"] = goDatatype
	fm["plural"] = inflect.Pluralize
	fm["gtt"] = codegen.GoTypeTransform
	fm["gttn"] = codegen.GoTypeTransformName
	return w.ExecuteTemplate("types", userTypeT, fm, data)
}

// arrayAttribute returns the array element attribute definition.
func arrayAttribute(a *design.AttributeDefinition) *design.AttributeDefinition {
	return a.Type.(*design.Array).ElemType
}

const (
	// userTypeT generates the code for a user type.
	// template input: UserTypeTemplateData
	userTypeT = `{{$ut := .UserType}}{{$ap := .AppPkg}}// {{if $ut.Description}}{{$ut.Description}} {{end}}
{{$ut.StructDefinition}}
// TableName overrides the table name settings in Gorm to force a specific table name
// in the database.
func (m {{$ut.Name}}) TableName() string {
{{ if ne $ut.Alias "" }}
return "{{ $ut.Alias}}" {{ else }} return "{{ $ut.TableName }}"
{{end}}
}
// {{$ut.Name}}DB is the implementation of the storage interface for
// {{$ut.Name}}.
type {{$ut.Name}}DB struct {
	Db gorm.DB
	log.Logger
	{{ if $ut.Cached }}cache *cache.Cache{{end}}
}
// New{{$ut.Name}}DB creates a new storage type.
func New{{$ut.Name}}DB(db gorm.DB, logger log.Logger) *{{$ut.Name}}DB {
	glog := logger.New("db", "{{$ut.Name}}")
	{{ if $ut.Cached }}return &{{$ut.Name}}DB{
		Db: db,
		Logger: glog,
		cache: cache.New(5*time.Minute, 30*time.Second),
	}
	{{ else  }}return &{{$ut.Name}}DB{Db: db, Logger: glog}{{ end  }}
}
// DB returns the underlying database.
func (m *{{$ut.Name}}DB) DB() interface{} {
	return &m.Db
}
{{ if $ut.Roler }}
// GetRole returns the value of the role field and satisfies the Roler interface.
func (m {{$ut.Name}}) GetRole() string {
	return {{$f := $ut.Fields.role}}{{if $f.Nullable}}*{{end}}m.Role
}
{{end}}

// {{$ut.Name}}Storage represents the storage interface.
type {{$ut.Name}}Storage interface {
	DB() interface{}
	List(ctx goa.Context{{ if $ut.DynamicTableName}}, tableName string{{ end }}) []{{$ut.Name}}
	One(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, {{$ut.PKAttributes}}) ({{$ut.Name}}, error)
	Add(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, {{$ut.LowerName}} {{$ut.Name}}) ({{$ut.Name}}, error)
	Update(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, {{$ut.LowerName}} {{$ut.Name}}) (error)
	Delete(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, {{ $ut.PKAttributes}}) (error) 	
}

// TableName overrides the table name settings in Gorm to force a specific table name
// in the database.
func (m *{{$ut.Name}}DB) TableName() string {
{{ if ne $ut.Alias "" }}
return "{{ $ut.Alias}}" {{ else }} return "{{ $ut.TableName }}"
{{end}}
}

{{ range $vname, $view := $ut.RenderTo.Views}}
// Transformation
{{ $mtd := $ut.Project $vname }}
{{$functionName := gttn $ut.RenderTo.UserTypeDefinition $mtd.UserTypeDefinition ""}}
{{ gtt $ut.RenderTo.UserTypeDefinition $mtd.UserTypeDefinition "app" $functionName }}

// CRUD Functions
// List{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}} returns an array of view: {{$vname}}
func (m *{{$ut.Name}}DB) List{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}} (ctx goa.Context{{ if $ut.DynamicTableName}}, tableName string{{ end }}) []app.{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}}{
	now := time.Now()
	defer ctx.Info("List{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}}", "duration", time.Since(now))
	var objs []app.{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}}
	err := m.Db.Table({{ if $ut.DynamicTableName }}.Table(tableName){{else}}m.TableName(){{ end }}).{{ range $ln, $lv := $ut.RenderTo.Links }}Preload("{{goify $ln true}}").{{end}}Find(&objs).Error
	if err != nil {
		ctx.Error("error listing {{$ut.Name}}", "error", err.Error())
		return objs
	}

	return objs
}

// One{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}} returns an array of view: {{$vname}}
func (m *{{$ut.Name}}DB) One{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}} (ctx goa.Context{{ if $ut.DynamicTableName}}, tableName string{{ end }}, id int) app.{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}}{	
	now := time.Now()
	defer ctx.Info("One{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}}", "duration", time.Since(now))
	var view app.{{$ut.RenderTo.TypeName}}{{if eq $vname "default"}}{{else}}{{goify $vname true}}{{end}}
	var native {{$ut.Name}}

	m.Db.Table({{ if $ut.DynamicTableName }}.Table(tableName){{else}}m.TableName(){{ end }}){{range $na, $hm:= $ut.HasMany}}.Preload("{{$hm.Name}}"){{end}}{{range $nm, $bt := $ut.BelongsTo}}.Preload("{{$bt.Name}}"){{end}}.Where("id = ?", id).Find(&native)
	fmt.Println(native)
	return view 
}
{{end}}
// Get{{$ut.Name}} returns a single {{$ut.Name}} as a Database Model
// This is more for use internally, and probably not what you want in  your controllers
func (m *{{$ut.Name}}DB) Get{{$ut.Name}}(ctx goa.Context{{ if $ut.DynamicTableName}}, tableName string{{ end }}, id int) {{$ut.Name}}{	
	now := time.Now()
	defer ctx.Info("Get{{$ut.Name}}", "duration", time.Since(now))
	var native {{$ut.Name}}
	m.Db.Table({{ if $ut.DynamicTableName }}.Table(tableName){{else}}m.TableName(){{ end }}).Where("id = ?", id).Find(&native)
	return native 
}
// Add creates a new record.
func (m *{{$ut.Name}}DB) Add(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, model {{$ut.Name}}) ({{$ut.Name}}, error) {
	now := time.Now()
	defer ctx.Info("Add{{$ut.Name}}", "duration", time.Since(now))
	err := m.Db{{ if $ut.DynamicTableName }}.Table(tableName){{ end }}.Create(&model).Error
	if err != nil {
		ctx.Error("error updating {{$ut.Name}}", "error", err.Error())
		return model, err
	}
	{{ if $ut.Cached }}
	go m.cache.Set(strconv.Itoa(model.ID), model, cache.DefaultExpiration) {{ end }}
	return model, err
}
// Update modifies a single record.
func (m *{{$ut.Name}}DB) Update(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, model {{$ut.Name}}) error {
	now := time.Now()
	defer ctx.Info("Update{{$ut.Name}}", "duration", time.Since(now))
	obj := m.Get{{$ut.Name}}(ctx{{ if $ut.DynamicTableName }}, tableName{{ end }}, {{$ut.PKUpdateFields "model"}})
	err := m.Db{{ if $ut.DynamicTableName }}.Table(tableName){{ end }}.Model(&obj).Updates(model).Error
	{{ if $ut.Cached }}go func(){
	obj, err := m.One(ctx, model.ID)
	if err == nil {
		m.cache.Set(strconv.Itoa(model.ID), obj, cache.DefaultExpiration)
	}
	}()
	{{ end }}
	return err
}
// Delete removes a single record.
func (m *{{$ut.Name}}DB) Delete(ctx goa.Context{{ if $ut.DynamicTableName }}, tableName string{{ end }}, {{$ut.PKAttributes}})  error {
	now := time.Now()
	defer ctx.Info("Delete{{$ut.Name}}", "duration", time.Since(now))
	var obj {{$ut.Name}}{{ $l := len $ut.PrimaryKeys }}
	{{ if eq $l 1 }}
	err := m.Db{{ if $ut.DynamicTableName }}.Table(tableName){{ end }}.Delete(&obj, id).Error
	{{ else  }}err := m.Db{{ if $ut.DynamicTableName }}.Table(tableName){{ end }}.Delete(&obj).Where("{{$ut.PKWhere}}", {{$ut.PKWhereFields}}).Error
	{{ end }}
	if err != nil {
		ctx.Error("error retrieving {{$ut.Name}}", "error", err.Error())
		return  err
	}
	{{ if $ut.Cached }} go m.cache.Delete(strconv.Itoa(id)) {{ end }}
	return  nil
}
`
)
