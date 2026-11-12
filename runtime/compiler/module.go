package compiler

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"go_js/chunk"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
)

type ModuleLinkError struct {
	Message string
}

func (e *ModuleLinkError) Error() string {
	return e.Message
}

type ModuleLoadError struct {
	Err error
}

func (e *ModuleLoadError) Error() string {
	return e.Err.Error()
}

type ModuleExport struct {
	Name          string
	Slot          int
	IsNamespace   bool
	NamespacePath string
}

type ModuleImport struct {
	Local     string
	Imported  string
	Source    string
	Namespace bool
}

type ModuleNamespaceImport struct {
	Local  string
	Source string
}

type ModuleRecord struct {
	Path             string
	AST              *parser.Node
	Scope            *FunctionScope
	Main             object.Callable
	MainValue        value.Value
	Exports          map[string]*Variable
	NamespaceExports map[string]*ModuleRecord
	Explicit         map[string]bool
	Ambiguous        map[string]bool
	Imports          []ModuleImport
	NamespaceImports []ModuleNamespaceImport
	Dependencies     []*ModuleRecord
	DependencyMap    map[string]*ModuleRecord
	StarExports      []*ModuleRecord
	LoadErr          error
	LinkErr          error
	Async            bool
	exportsBuilt     bool
	exportsBuilding  bool
}

var moduleRecords = map[string]*ModuleRecord{}
var entryModule *ModuleRecord
var entryFilePath string
var nextModuleGlobalSlot int
var compilingModulePath string

func SetEntryFile(path string) {
	entryFilePath = path
}

func ResolveModulePath(specifier, parent string) string {
	if strings.HasPrefix(specifier, ".") {
		return path.Clean(path.Join(path.Dir(parent), specifier))
	}
	return path.Clean(specifier)
}

func moduleSource(node *parser.Node) string {
	if node == nil || node.Source == nil {
		return ""
	}
	if source, ok := node.Source.Value.([]byte); ok {
		return string(source)
	}
	return ""
}

func loadModuleRecord(modulePath string, ast *parser.Node) *ModuleRecord {
	if record, found := moduleRecords[modulePath]; found {
		return record
	}
	record := &ModuleRecord{
		Path:             modulePath,
		AST:              ast,
		Exports:          map[string]*Variable{},
		NamespaceExports: map[string]*ModuleRecord{},
		Explicit:         map[string]bool{},
		Ambiguous:        map[string]bool{},
		DependencyMap:    map[string]*ModuleRecord{},
	}
	moduleRecords[modulePath] = record
	if record.AST == nil {
		data, err := os.ReadFile(modulePath)
		if err != nil {
			record.LoadErr = &ModuleLoadError{Err: err}
			return record
		}
		parsed, err := parser.GetAst(data, &parser.Options{SourceType: "module"}, 0)
		if err != nil {
			record.LoadErr = &ModuleLoadError{Err: err}
			return record
		}
		record.AST = parsed
	}
	if record.AST == nil {
		return record
	}
	record.Async = moduleHasTopLevelAwait(record.AST)
	for _, node := range record.AST.Body {
		var source string
		switch node.Type {
		case parser.NODE_IMPORT_DECLARATION:
			source = moduleSource(node)
			for _, specifier := range node.Specifiers {
				imported := ""
				namespace := false
				switch specifier.Type {
				case parser.NODE_IMPORT_DEFAULT_SPECIFIER:
					imported = "default"
				case parser.NODE_IMPORT_NAMESPACE_SPECIFIER:
					namespace = true
					imported = "*"
				case parser.NODE_IMPORT_SPECIFIER:
					if specifier.Imported != nil {
						imported = specifier.Imported.Name
					}
				}
				local := ""
				if specifier.Local != nil {
					local = specifier.Local.Name
				}
				record.Imports = append(record.Imports, ModuleImport{Local: local, Imported: imported, Source: source, Namespace: namespace})
			}
		case parser.NODE_EXPORT_NAMED_DECLARATION:
			if node.Source != nil {
				source = moduleSource(node)
			}
		case parser.NODE_EXPORT_ALL_DECLARATION:
			source = moduleSource(node)
		}
		if source == "" {
			continue
		}
		dependencyPath := ResolveModulePath(source, modulePath)
		dependency, found := record.DependencyMap[dependencyPath]
		if !found {
			dependency = loadModuleRecord(dependencyPath, nil)
			record.DependencyMap[dependencyPath] = dependency
			record.Dependencies = append(record.Dependencies, dependency)
		}
	}
	return record
}

func moduleHasTopLevelAwait(node *parser.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case parser.NODE_FUNCTION_DECLARATION, parser.NODE_FUNCTION_EXPRESSION, parser.NODE_ARROW_FUNCTION_EXPRESSION, parser.NODE_METHOD_DEFINITION, parser.NODE_PROPERTY_DEFINITION, parser.NODE_STATIC_BLOCK:
		return false
	case parser.NODE_AWAIT_EXPRESSION:
		return true
	}
	if moduleHasTopLevelAwait(node.Declaration) || moduleHasTopLevelAwait(node.Expression) || moduleHasTopLevelAwait(node.Argument) || moduleHasTopLevelAwait(node.Left) || moduleHasTopLevelAwait(node.Right) || moduleHasTopLevelAwait(node.Test) || moduleHasTopLevelAwait(node.Consequent) || moduleHasTopLevelAwait(node.Alternate) || moduleHasTopLevelAwait(node.Initializer) || moduleHasTopLevelAwait(node.Callee) || moduleHasTopLevelAwait(node.Object) || moduleHasTopLevelAwait(node.Property) || moduleHasTopLevelAwait(node.BodyNode) || moduleHasTopLevelAwait(node.Block) || moduleHasTopLevelAwait(node.Finalizer) {
		return true
	}
	for _, item := range node.Body {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	for _, item := range node.Declarations {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	for _, item := range node.Arguments {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	for _, item := range node.Elements {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	for _, item := range node.Properties {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	for _, item := range node.Expressions {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	for _, item := range node.Cases {
		if moduleHasTopLevelAwait(item) {
			return true
		}
	}
	return false
}

func moduleDeclarationExports(record *ModuleRecord, declaration *parser.Node) {
	if declaration == nil {
		return
	}
	switch declaration.Type {
	case parser.NODE_VARIABLE_DECLARATION:
		for _, declarator := range declaration.Declarations {
			if declarator.Identifier == nil {
				continue
			}
			name := declarator.Identifier.Name
			if variable, _ := record.Scope.findVariable(name); variable != nil {
				record.Exports[name] = variable
			}
		}
	case parser.NODE_FUNCTION_DECLARATION, parser.NODE_CLASS_DECLARATION:
		if declaration.Identifier != nil {
			name := declaration.Identifier.Name
			if variable, _ := record.Scope.findVariable(name); variable != nil {
				record.Exports[name] = variable
			}
		}
	}
}

func (record *ModuleRecord) importForLocal(local string) (ModuleImport, bool) {
	for _, binding := range record.Imports {
		if binding.Local == local {
			return binding, true
		}
	}
	return ModuleImport{}, false
}

func (record *ModuleRecord) localBinding(name string, stack map[*ModuleRecord]bool) (*Variable, bool, error) {
	if variable, _ := record.Scope.findVariable(name); variable != nil {
		return variable, true, nil
	}
	if binding, found := record.importForLocal(name); found {
		if binding.Namespace {
			return nil, false, nil
		}
		dependency := record.DependencyMap[ResolveModulePath(binding.Source, record.Path)]
		if dependency == nil {
			return nil, false, &ModuleLinkError{Message: fmt.Sprintf("Cannot resolve module '%s'", binding.Source)}
		}
		if err := buildModuleExports(dependency, stack); err != nil {
			return nil, false, err
		}
		if variable, found := dependency.Exports[binding.Imported]; found {
			return variable, true, nil
		}
	}
	return nil, false, nil
}

func buildModuleExports(record *ModuleRecord, stack map[*ModuleRecord]bool) error {
	if record.exportsBuilt {
		return record.LinkErr
	}
	if record.exportsBuilding {
		return nil
	}
	record.exportsBuilding = true
	stack[record] = true
	defer delete(stack, record)

	if record.LoadErr != nil {
		record.LinkErr = record.LoadErr
		record.exportsBuilt = true
		return record.LinkErr
	}

	for _, node := range record.AST.Body {
		switch node.Type {
		case parser.NODE_EXPORT_NAMED_DECLARATION:
			if node.Declaration != nil {
				moduleDeclarationExports(record, node.Declaration)
				for name := range record.Exports {
					record.Explicit[name] = true
				}
				continue
			}
			for _, specifier := range node.Specifiers {
				if specifier.Exported == nil || specifier.Local == nil {
					continue
				}
				exported := specifier.Exported.Name
				record.Explicit[exported] = true
				if node.Source != nil {
					dependency := record.DependencyMap[ResolveModulePath(moduleSource(node), record.Path)]
					if dependency == nil {
						record.LinkErr = &ModuleLinkError{Message: "Cannot resolve re-export module"}
						continue
					}
					if err := buildModuleExports(dependency, stack); err != nil {
						record.LinkErr = err
						continue
					}
					if variable, found := dependency.Exports[specifier.Local.Name]; found {
						record.Exports[exported] = variable
					} else if namespace, found := dependency.NamespaceExports[specifier.Local.Name]; found {
						record.NamespaceExports[exported] = namespace
					} else {
						record.LinkErr = &ModuleLinkError{Message: fmt.Sprintf("The requested module '%s' does not provide an export named '%s'", moduleSource(node), specifier.Local.Name)}
					}
				} else {
					variable, found, err := record.localBinding(specifier.Local.Name, stack)
					if err != nil {
						record.LinkErr = err
					} else if found {
						record.Exports[exported] = variable
					} else if namespace, ok := record.NamespaceExports[specifier.Local.Name]; ok {
						record.NamespaceExports[exported] = namespace
					} else {
						record.LinkErr = &ModuleLinkError{Message: fmt.Sprintf("Export '%s' is not defined", specifier.Local.Name)}
					}
				}
			}
		case parser.NODE_EXPORT_DEFAULT_DECLARATION:
			record.Explicit["default"] = true
			if node.Declaration != nil {
				if node.Declaration.Identifier != nil {
					name := node.Declaration.Identifier.Name
					if variable, _ := record.Scope.findVariable(name); variable != nil {
						record.Exports["default"] = variable
					}
				}
			} else if variable, found := DEFAULT_EXPORT_VARS[node]; found {
				record.Exports["default"] = variable
			}
		case parser.NODE_EXPORT_ALL_DECLARATION:
			dependency := record.DependencyMap[ResolveModulePath(moduleSource(node), record.Path)]
			if dependency == nil {
				record.LinkErr = &ModuleLinkError{Message: "Cannot resolve star export module"}
				continue
			}
			if node.Exported != nil {
				record.Explicit[node.Exported.Name] = true
				if err := buildModuleExports(dependency, stack); err != nil {
					record.LinkErr = err
					continue
				}
				record.NamespaceExports[node.Exported.Name] = dependency
			} else {
				record.StarExports = append(record.StarExports, dependency)
			}
		}
	}

	for _, dependency := range record.StarExports {
		if err := buildModuleExports(dependency, stack); err != nil {
			if record.LinkErr == nil {
				record.LinkErr = err
			}
			continue
		}
		for name, variable := range dependency.Exports {
			if name == "default" || record.Explicit[name] {
				continue
			}
			if existing, found := record.Exports[name]; found {
				if existing != variable {
					delete(record.Exports, name)
					record.Ambiguous[name] = true
				}
			} else if !record.Ambiguous[name] {
				record.Exports[name] = variable
			}
		}
	}

	record.exportsBuilding = false
	record.exportsBuilt = true
	return record.LinkErr
}

func linkModuleImports(record *ModuleRecord) {
	for _, binding := range record.Imports {
		dependency := record.DependencyMap[ResolveModulePath(binding.Source, record.Path)]
		if dependency == nil {
			if record.LinkErr == nil {
				record.LinkErr = &ModuleLinkError{Message: fmt.Sprintf("Cannot resolve module '%s'", binding.Source)}
			}
			continue
		}
		if err := buildModuleExports(dependency, map[*ModuleRecord]bool{}); err != nil {
			if record.LinkErr == nil {
				record.LinkErr = err
			}
			continue
		}
		if binding.Namespace {
			if variable, found := record.Scope.vars[binding.Local]; found {
				variable.declared = true
			} else {
				variable := &Variable{scope: GLOBAL, type_: CONST, slot: nextModuleGlobalSlot, declared: true, owner: record.Scope}
				record.Scope.vars[binding.Local] = variable
				record.Scope.varOrder = append(record.Scope.varOrder, variable)
				record.Scope.maxSlot = nextModuleGlobalSlot
				nextModuleGlobalSlot++
			}
			record.NamespaceImports = append(record.NamespaceImports, ModuleNamespaceImport{Local: binding.Local, Source: dependency.Path})
			continue
		}
		variable, found := dependency.Exports[binding.Imported]
		if !found {
			if record.LinkErr == nil {
				record.LinkErr = &ModuleLinkError{Message: fmt.Sprintf("The requested module '%s' does not provide an export named '%s'", binding.Source, binding.Imported)}
			}
			continue
		}
		record.Scope.vars[binding.Local] = &Variable{scope: GLOBAL, type_: CONST, slot: variable.slot, declared: true, owner: record.Scope}
	}
}

func compileModuleRecord(record *ModuleRecord) error {
	if record.LoadErr != nil {
		return record.LoadErr
	}
	if record.LinkErr != nil {
		return record.LinkErr
	}
	savedModule := module
	savedStrict := strictContext
	savedModulePath := compilingModulePath
	module = true
	strictContext = true
	compilingModulePath = record.Path

	var main object.Callable
	if record.Async {
		main = native.NewAsyncFunction(fmt.Sprintf("MODULE %s", record.Path), 0, nil)
	} else {
		main = object.NewFunction(fmt.Sprintf("MODULE %s", record.Path), 0, nil)
		main.(*object.ObjFunction).Strict = true
	}
	record.Main = main

	for _, variable := range record.Scope.varOrder {
		if variable.owner != record.Scope {
			continue
		}
		main.ValueChunk().EmitByte(chunk.OP_PUSH_UNINITIALIZED)
		main.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL, uint8(variable.slot))
	}
	for _, dependency := range record.Dependencies {
		main.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(dependency.Path))))
		main.ValueChunk().EmitByte(chunk.OP_IMPORT_MODULE)
		if record.Async {
			main.ValueChunk().EmitByte(chunk.OP_AWAIT)
		}
		main.ValueChunk().EmitByte(chunk.OP_POP)
	}
	for _, namespaceImport := range record.NamespaceImports {
		variable := record.Scope.vars[namespaceImport.Local]
		main.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(namespaceImport.Source))))
		main.ValueChunk().EmitByte(chunk.OP_GET_MODULE_NAMESPACE)
		main.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL, uint8(variable.slot))
	}
	generateByteCode(record.AST, record.Scope, main)
	main.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(record.Path))))
	main.ValueChunk().EmitByte(chunk.OP_GET_MODULE_NAMESPACE)
	main.ValueChunk().EmitByte(chunk.OP_RETURN)
	record.MainValue = value.EncodeHandle(heap.Allocate(main))
	heap.PushToRoots(record.MainValue)

	module = savedModule
	strictContext = savedStrict
	compilingModulePath = savedModulePath
	return nil
}

func compileModuleGraph(ast *parser.Node) (*ModuleRecord, error) {
	moduleRecords = map[string]*ModuleRecord{}
	entryPath := path.Clean(entryFilePath)
	entry := loadModuleRecord(entryPath, ast)
	records := make([]*ModuleRecord, 0, len(moduleRecords))
	for _, record := range moduleRecords {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	nextModuleGlobalSlot = len(global.varOrder)
	for _, record := range records {
		if record.LoadErr != nil {
			continue
		}
		scope := newFunctionScope(global, GLOBAL)
		scope.globalBase = nextModuleGlobalSlot
		scope.maxSlot = nextModuleGlobalSlot
		record.Scope = scope
		prePass(record.AST, scope)
		if scope.maxSlot+1 > nextModuleGlobalSlot {
			nextModuleGlobalSlot = scope.maxSlot + 1
		}
	}
	for _, record := range records {
		if record.LoadErr != nil {
			continue
		}
		if err := buildModuleExports(record, map[*ModuleRecord]bool{}); err != nil && record == entry {
			return entry, err
		}
	}
	for _, record := range records {
		if record.LoadErr != nil {
			continue
		}
		linkModuleImports(record)
		if record == entry && record.LinkErr != nil {
			return entry, record.LinkErr
		}
	}
	for _, record := range records {
		if record.LoadErr != nil {
			if record == entry {
				return entry, record.LoadErr
			}
			continue
		}
		if record.LinkErr != nil {
			if record == entry {
				return entry, record.LinkErr
			}
			continue
		}
		for _, dependency := range record.Dependencies {
			if dependency.Async {
				record.Async = true
			}
		}
	}
	for _, record := range records {
		if record.LoadErr != nil || record.LinkErr != nil {
			continue
		}
		if err := compileModuleRecord(record); err != nil {
			if record == entry {
				return entry, err
			}
		}
	}
	entryModule = entry
	return entry, nil
}

func EntryModule() *ModuleRecord {
	return entryModule
}

func ModuleRecords() []*ModuleRecord {
	records := make([]*ModuleRecord, 0, len(moduleRecords))
	for _, record := range moduleRecords {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	return records
}

func LoadModule(specifier, parent string) (*ModuleRecord, error) {
	modulePath := ResolveModulePath(specifier, parent)
	record, found := moduleRecords[modulePath]
	if !found {
		record = loadModuleRecord(modulePath, nil)
		records := make([]*ModuleRecord, 0)
		for _, loaded := range moduleRecords {
			records = append(records, loaded)
		}
		sort.Slice(records, func(i, j int) bool {
			return records[i].Path < records[j].Path
		})
		for _, loaded := range records {
			if loaded.LoadErr != nil || loaded.Scope != nil {
				continue
			}
			scope := newFunctionScope(global, GLOBAL)
			scope.globalBase = nextModuleGlobalSlot
			scope.maxSlot = nextModuleGlobalSlot
			loaded.Scope = scope
			prePass(loaded.AST, scope)
			if scope.maxSlot+1 > nextModuleGlobalSlot {
				nextModuleGlobalSlot = scope.maxSlot + 1
			}
		}
		for _, loaded := range records {
			if loaded.LoadErr != nil || loaded.Scope == nil {
				continue
			}
			if err := buildModuleExports(loaded, map[*ModuleRecord]bool{}); err != nil {
				if loaded == record {
					return record, err
				}
			}
		}
		for _, loaded := range records {
			if loaded.LoadErr != nil || loaded.Scope == nil {
				continue
			}
			linkModuleImports(loaded)
			if loaded == record && loaded.LinkErr != nil {
				return record, loaded.LinkErr
			}
		}
		for _, loaded := range records {
			if loaded.LoadErr != nil || loaded.LinkErr != nil {
				continue
			}
			for _, dependency := range loaded.Dependencies {
				if dependency.Async {
					loaded.Async = true
				}
			}
		}
		for _, loaded := range records {
			if loaded.LoadErr != nil || loaded.LinkErr != nil || loaded.Main != nil {
				continue
			}
			if err := compileModuleRecord(loaded); err != nil {
				if loaded == record {
					return record, err
				}
			}
		}
	}
	if record.LoadErr != nil {
		return record, record.LoadErr
	}
	if record.LinkErr != nil {
		return record, record.LinkErr
	}
	return record, nil
}

func (record *ModuleRecord) ExportDescriptors() []ModuleExport {
	names := make([]string, 0, len(record.Exports)+len(record.NamespaceExports))
	seen := map[string]bool{}
	for name := range record.Exports {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for name := range record.NamespaceExports {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	descriptors := make([]ModuleExport, 0, len(names))
	for _, name := range names {
		if variable, found := record.Exports[name]; found {
			descriptors = append(descriptors, ModuleExport{Name: name, Slot: variable.slot})
		} else if namespace, found := record.NamespaceExports[name]; found {
			descriptors = append(descriptors, ModuleExport{Name: name, IsNamespace: true, NamespacePath: namespace.Path})
		}
	}
	return descriptors
}

func ModuleHasTopLevelAwait(node *parser.Node) bool {
	return moduleHasTopLevelAwait(node)
}

var _ = strconv.Itoa
