package di

import (
	"fmt"
	"reflect"
	"strings"
)

// Filter defines a condition that a provider's metadata must satisfy.
type Filter interface {
	AppliesTo(key string) bool
	Matches(value interface{}) bool
}

// Provider represents a dependency provider with metadata and scope.
type Provider struct {
	fn         interface{}            // Factory function, e.g., func(dep1, dep2) T
	provides   reflect.Type           // Type it provides
	ParamTypes []reflect.Type         // Types of the parameters
	Metadata   map[string]interface{} // Metadata for filtering
	scope      string                 // "singleton" or "prototype"
}

// Container manages providers, meta-providers, and filters.
type Container struct {
	Providers      map[reflect.Type][]Provider
	metaProviders  []MetaProvider
	filters        map[string]func(string, string) Filter // Updated: filterType -> factory(key, param)
	singletonCache map[reflect.Type]map[*Provider]interface{}
	resolving      map[reflect.Type]bool // For cycle detection
}

// MetaProvider is a function that registers providers into the container.
type MetaProvider func(c *Container)

// NewContainer initializes a DI container with default filters.
func NewContainer() *Container {
	c := &Container{
		Providers:      make(map[reflect.Type][]Provider),
		filters:        make(map[string]func(string, string) Filter),
		singletonCache: make(map[reflect.Type]map[*Provider]interface{}),
		resolving:      make(map[reflect.Type]bool),
	}
	// Register default filters with new syntax
	c.RegisterFilter("versionGreaterThan", func(key, param string) Filter {
		return &VersionGreaterThanFilter{key: key, version: param}
	})
	c.RegisterFilter("nameEquals", func(key, param string) Filter {
		return &NameEqualsFilter{key: key, name: param}
	})
	return c
}

// RegisterMetaProvider adds a meta-provider for runtime provider registration.
func (c *Container) RegisterMetaProvider(mp MetaProvider) {
	c.metaProviders = append(c.metaProviders, mp)
}

// Build invokes all meta-providers to register their providers.
func (c *Container) Build() {
	for _, mp := range c.metaProviders {
		mp(c)
	}
}

// RegisterProvider registers a provider for type T with metadata and scope.
func (c *Container) RegisterProvider(fn interface{}, metadata map[string]interface{}, scope string) {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic("provider must be a function")
	}
	if fnType.NumOut() != 1 {
		panic("provider must return exactly one value")
	}
	returnType := fnType.Out(0)
	paramTypes := make([]reflect.Type, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		paramTypes[i] = fnType.In(i)
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	provider := Provider{
		fn:         fn,
		provides:   returnType,
		ParamTypes: paramTypes,
		Metadata:   metadata,
		scope:      scope,
	}
	c.Providers[returnType] = append(c.Providers[returnType], provider)

	// Initialize the singleton cache for this type
	if scope == "singleton" {
		if _, ok := c.singletonCache[returnType]; !ok {
			c.singletonCache[returnType] = make(map[*Provider]interface{})
		}
	}
}

// Provide resolves a dependency of type T with optional filters.
func Provide[T any](c *Container, filters []Filter) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()
	val, err := c.provide(t, filters)
	if err != nil {
		return zero, err
	}
	return val.(T), nil
}

// provide is an internal method to resolve a dependency using reflection.
func (c *Container) provide(t reflect.Type, filters []Filter) (interface{}, error) {
	if c.resolving[t] {
		return nil, fmt.Errorf("cycle detected: type %v is already being resolved", t)
	}
	c.resolving[t] = true
	defer delete(c.resolving, t)

	// For struct types, use autoResolveStruct
	if t.Kind() == reflect.Struct {
		// Initialize singleton cache if needed
		if c.singletonCache == nil {
			c.singletonCache = make(map[reflect.Type]map[*Provider]interface{})
		}
		if c.singletonCache[t] == nil {
			c.singletonCache[t] = make(map[*Provider]interface{})
		}

		// Check cache for singleton instances
		if instance, exists := c.singletonCache[t][nil]; exists {
			return instance, nil
		}

		// Resolve the struct
		instance, err := c.autoResolveStruct(t, filters)
		if err != nil {
			return nil, err
		}

		// Note: caching is handled inside autoResolveStruct based on provider scopes
		return instance, nil
	}

	// For pointer to struct types, use autoResolveStruct
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		// Initialize singleton cache if needed
		if c.singletonCache == nil {
			c.singletonCache = make(map[reflect.Type]map[*Provider]interface{})
		}
		if c.singletonCache[t] == nil {
			c.singletonCache[t] = make(map[*Provider]interface{})
		}

		// Check cache for singleton instances
		if instance, exists := c.singletonCache[t][nil]; exists {
			return instance, nil
		}

		// Resolve the struct
		structVal, err := c.autoResolveStruct(t.Elem(), filters)
		if err != nil {
			return nil, err
		}

		// Create a pointer to the struct
		ptrVal := reflect.New(t.Elem())
		ptrVal.Elem().Set(reflect.ValueOf(structVal))
		instance := ptrVal.Interface()

		// Note: We don't cache here because caching is handled in autoResolveStruct based on provider scopes

		return instance, nil
	}

	providers, exists := c.Providers[t]
	if !exists {
		return nil, fmt.Errorf("no di.Providers for type %v", t)
	}

	// Select first matching provider
	var provider *Provider
	for i := range providers {
		if c.providerMatchesFilters(providers[i], filters) {
			provider = &providers[i]
			break
		}
	}

	// If no provider matches the filters, return an error
	if provider == nil && len(filters) > 0 {
		return nil, fmt.Errorf("no di.Provider found matching constraints for type %v", t)
	}

	// Default to the first provider if no filters or no matches
	if provider == nil {
		provider = &providers[0]
	}

	// Check singleton cache
	if provider.scope == "singleton" {
		if c.singletonCache[t] == nil {
			c.singletonCache[t] = make(map[*Provider]interface{})
		}
		if instance, exists := c.singletonCache[t][provider]; exists {
			return instance, nil
		}
	}

	// Invoke provider
	instance, err := c.invokeProvider(*provider)
	if err != nil {
		return nil, err
	}

	// Cache singleton
	if provider.scope == "singleton" {
		c.singletonCache[t][provider] = instance
	}

	return instance, nil
}

// providerMatchesFilters checks if a provider's metadata satisfies all filters.
func (c *Container) providerMatchesFilters(p Provider, filters []Filter) bool {
	for _, f := range filters {
		matched := false
		for key, value := range p.Metadata {
			if f.AppliesTo(key) && f.Matches(value) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// invokeProvider calls the provider's factory function with resolved parameters.
func (c *Container) invokeProvider(p Provider) (interface{}, error) {
	fnValue := reflect.ValueOf(p.fn)
	paramValues := make([]reflect.Value, len(p.ParamTypes))
	for i, paramType := range p.ParamTypes {
		param, err := c.provide(paramType, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter %d of type %v: %w", i, paramType, err)
		}
		paramValues[i] = reflect.ValueOf(param)
	}
	results := fnValue.Call(paramValues)
	if len(results) != 1 {
		return nil, fmt.Errorf("provider function must return exactly one value")
	}
	return results[0].Interface(), nil
}

// RegisterFilter adds a custom filter type to the container.
func (c *Container) RegisterFilter(filterType string, factory func(key, param string) Filter) {
	c.filters[filterType] = factory
}

// FieldConstraint holds the constraints for a struct field.
type FieldConstraint struct {
	Field        reflect.StructField
	Literals     map[string]string
	Placeholders map[string]string
	Filters      []Filter
}

// ParseTag parses the DI tag into literals, placeholders, and filters.
func (c *Container) ParseTag(tag string) (literals map[string]string, placeholders map[string]string, filters []Filter, err error) {
	literals = make(map[string]string)
	placeholders = make(map[string]string)
	filters = []Filter{}
	if tag == "" {
		return
	}
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			err = fmt.Errorf("invalid tag part: %s", part)
			return
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		if strings.Contains(value, "(") && strings.HasSuffix(value, ")") {
			// Filter specification: e.g., "versionGreaterThan(1.0.0)"
			filterTypeEnd := strings.Index(value, "(")
			filterType := value[:filterTypeEnd]
			param := value[filterTypeEnd+1 : len(value)-1]
			if factory, ok := c.filters[filterType]; ok {
				filter := factory(key, param)
				filters = append(filters, filter)
			} else {
				err = fmt.Errorf("unknown di.Filter type: %s", filterType)
				return
			}
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			// Quoted literal string: e.g., "env='production'"
			literalValue := strings.Trim(value, "'")
			literals[key] = literalValue
		} else if strings.HasPrefix(value, "$") {
			// Placeholder: e.g., "env=$env"
			placeholders[key] = value[1:]
		} else {
			// Unquoted literal: e.g., "env=production"
			literals[key] = value
		}
	}
	return
}

// selectProviders selects providers for all fields that satisfy the constraints.
func selectProviders(c *Container, fields []FieldConstraint) (map[string]*Provider, error) {
	assignment := make(map[string]map[string]string)
	selected := make(map[string]*Provider)
	err := tryAssign(c, fields, assignment, selected, 0)
	if err != nil {
		return nil, err
	}
	return selected, nil
}

// tryAssign recursively assigns providers to fields using backtracking.
func tryAssign(c *Container, fields []FieldConstraint, assignment map[string]map[string]string, selected map[string]*Provider, index int) error {
	if index == len(fields) {
		return nil // All fields assigned
	}
	field := fields[index]
	fieldType := field.Field.Type

	// Handle structs and pointers to structs
	if fieldType.Kind() == reflect.Struct || (fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct) {
		selected[field.Field.Name] = nil // Mark for auto-resolution
		return tryAssign(c, fields, assignment, selected, index+1)
	}

	providers, ok := c.Providers[fieldType]
	if !ok {
		return fmt.Errorf("no providers for type %v", fieldType)
	}

	for _, p := range providers {
		// Check literals
		satisfiesLiterals := true
		for key, literal := range field.Literals {
			if val, ok := p.Metadata[key].(string); !ok || val != literal {
				satisfiesLiterals = false
				break
			}
		}
		if !satisfiesLiterals {
			continue
		}
		// Check placeholders
		satisfiesPlaceholders := true
		for key, placeholder := range field.Placeholders {
			if assignedVal, ok := assignment[key][placeholder]; ok {
				if val, ok := p.Metadata[key].(string); !ok || val != assignedVal {
					satisfiesPlaceholders = false
					break
				}
			}
		}
		if !satisfiesPlaceholders {
			continue
		}
		// Check filters
		if !c.providerMatchesFilters(p, field.Filters) {
			continue
		}
		// Assign provider
		newAssignment := copyAssignment(assignment)
		for key, placeholder := range field.Placeholders {
			if _, ok := newAssignment[key]; !ok {
				newAssignment[key] = make(map[string]string)
			}
			if _, ok := newAssignment[key][placeholder]; !ok {
				if val, ok := p.Metadata[key].(string); ok {
					newAssignment[key][placeholder] = val
				} else {
					continue
				}
			}
		}
		selected[field.Field.Name] = &p
		if err := tryAssign(c, fields, newAssignment, selected, index+1); err == nil {
			return nil
		}
		delete(selected, field.Field.Name) // Backtrack
	}
	return fmt.Errorf("no suitable provider for field %s", field.Field.Name)
}

// copyAssignment creates a deep copy of the assignment map.
func copyAssignment(assignment map[string]map[string]string) map[string]map[string]string {
	newAssignment := make(map[string]map[string]string)
	for key, inner := range assignment {
		newInner := make(map[string]string)
		for k, v := range inner {
			newInner[k] = v
		}
		newAssignment[key] = newInner
	}
	return newAssignment
}

// ProvideStruct creates and populates a struct of type T based on DI tags.
func ProvideStruct[T any](c *Container) (T, error) {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() != reflect.Struct {
		return zero, fmt.Errorf("ProvideStruct requires a struct type, got %v", t)
	}

	// Initialize singleton cache if needed
	if c.singletonCache == nil {
		c.singletonCache = make(map[reflect.Type]map[*Provider]interface{})
	}
	if c.singletonCache[t] == nil {
		c.singletonCache[t] = make(map[*Provider]interface{})
	}

	// Return cached instance if exists (only for singleton scope)
	if instance, exists := c.singletonCache[t][nil]; exists {
		return instance.(T), nil
	}

	// Collect field constraints
	var fieldConstraints []FieldConstraint
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("di")
		literals, placeholders, filters, err := c.ParseTag(tag)
		if err != nil {
			return zero, fmt.Errorf("error parsing DI tag for field %s: %w", field.Name, err)
		}
		fieldConstraints = append(fieldConstraints, FieldConstraint{
			Field:        field,
			Literals:     literals,
			Placeholders: placeholders,
			Filters:      filters,
		})
	}

	// Select providers for each field
	selectedProviders, err := selectProviders(c, fieldConstraints)
	if err != nil {
		return zero, err
	}

	// Check if any provider is prototype scope
	hasPrototypeProvider := false
	for _, p := range selectedProviders {
		if p != nil && p.scope == "prototype" {
			hasPrototypeProvider = true
			break
		}
	}

	// Create and populate struct instance
	v := reflect.New(t).Elem()
	for _, fc := range fieldConstraints {
		fieldType := fc.Field.Type
		fieldValue := v.FieldByIndex(fc.Field.Index)

		if provider, exists := selectedProviders[fc.Field.Name]; exists && provider != nil {
			instance, err := c.invokeProvider(*provider)
			if err != nil {
				return zero, fmt.Errorf("failed to resolve field %s: %w", fc.Field.Name, err)
			}
			fieldValue.Set(reflect.ValueOf(instance))
		} else if fieldType.Kind() == reflect.Struct {
			nestedInstance, err := c.autoResolveStruct(fieldType, fc.Filters)
			if err != nil {
				return zero, fmt.Errorf("failed to resolve nested struct field %s: %w", fc.Field.Name, err)
			}
			fieldValue.Set(reflect.ValueOf(nestedInstance))
		} else if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			nestedInstance, err := c.autoResolveStruct(fieldType.Elem(), fc.Filters)
			if err != nil {
				return zero, fmt.Errorf("failed to resolve nested struct pointer field %s: %w", fc.Field.Name, err)
			}
			ptrToInstance := reflect.New(fieldType.Elem())
			ptrToInstance.Elem().Set(reflect.ValueOf(nestedInstance))
			fieldValue.Set(ptrToInstance)
		} else {
			return zero, fmt.Errorf("no provider found for field %s of type %v", fc.Field.Name, fieldType)
		}
	}

	result := v.Interface().(T)

	// Only cache if all providers are singleton
	if !hasPrototypeProvider {
		c.singletonCache[t][nil] = result
	}

	return result, nil
}

// autoResolveStruct automatically resolves a struct type using reflection.
func (c *Container) autoResolveStruct(t reflect.Type, filters []Filter) (interface{}, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("autoResolveStruct requires a struct type, got %v", t)
	}

	var fieldConstraints []FieldConstraint
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("di")
		literals, placeholders, filters, err := c.ParseTag(tag)
		if err != nil {
			return nil, fmt.Errorf("error parsing DI tag for field %s: %w", field.Name, err)
		}
		fieldConstraints = append(fieldConstraints, FieldConstraint{
			Field:        field,
			Literals:     literals,
			Placeholders: placeholders,
			Filters:      filters,
		})
	}

	selectedProviders, err := selectProviders(c, fieldConstraints)
	if err != nil {
		return nil, err
	}

	// Create and populate struct instance
	v := reflect.New(t).Elem()

	// Track if any provider is prototype scope to determine caching behavior
	hasPrototypeProvider := false

	for _, fc := range fieldConstraints {
		fieldType := fc.Field.Type
		// Check if field is a struct or pointer to struct
		if fieldType.Kind() == reflect.Struct {
			fieldVal, err := c.autoResolveStruct(fieldType, fc.Filters)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve field %s: %w", fc.Field.Name, err)
			}
			v.FieldByName(fc.Field.Name).Set(reflect.ValueOf(fieldVal))
		} else if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			if fieldVal, err := c.autoResolveStruct(fieldType.Elem(), fc.Filters); err == nil {
				ptrVal := reflect.New(fieldType.Elem())
				ptrVal.Elem().Set(reflect.ValueOf(fieldVal))
				v.FieldByName(fc.Field.Name).Set(ptrVal)
			} else {
				return nil, fmt.Errorf("failed to resolve field %s: %w", fc.Field.Name, err)
			}
		} else if p, ok := selectedProviders[fc.Field.Name]; ok {
			var instance interface{}
			var err error

			if p == nil {
				// Auto-resolve field using struct or pointer type
				instance, err = c.provide(fieldType, fc.Filters)
			} else {
				// Invoke specific provider
				instance, err = c.invokeProvider(*p)

				// Check if this is a prototype provider
				if p.scope == "prototype" {
					hasPrototypeProvider = true
				}
			}

			if err != nil {
				return nil, fmt.Errorf("failed to resolve field %s: %w", fc.Field.Name, err)
			}

			fieldValue := reflect.ValueOf(instance)
			v.FieldByName(fc.Field.Name).Set(fieldValue)
		} else {
			return nil, fmt.Errorf("no provider found for field %s of type %v", fc.Field.Name, fieldType)
		}
	}

	result := v.Interface()

	// Only cache if all providers are singleton
	if !hasPrototypeProvider {
		if c.singletonCache == nil {
			c.singletonCache = make(map[reflect.Type]map[*Provider]interface{})
		}
		if c.singletonCache[t] == nil {
			c.singletonCache[t] = make(map[*Provider]interface{})
		}
		c.singletonCache[t][nil] = result
	}

	return result, nil
}

// Filter Implementations

type VersionGreaterThanFilter struct {
	key     string
	version string
}

func (f *VersionGreaterThanFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *VersionGreaterThanFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v > f.version
	}
	return false
}

type NameEqualsFilter struct {
	key  string
	name string
}

func (f *NameEqualsFilter) AppliesTo(key string) bool {
	return key == f.key
}

func (f *NameEqualsFilter) Matches(value interface{}) bool {
	if v, ok := value.(string); ok {
		return v == f.name
	}
	return false
}
