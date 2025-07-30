package model

import "encoding/json"

// DeepCopy crea una copia profunda de un Node
func (n *Node) DeepCopy() (*Node, error) {
	// Usar JSON para hacer una copia profunda
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	
	var copy Node
	err = json.Unmarshal(data, &copy)
	if err != nil {
		return nil, err
	}
	
	return &copy, nil
}

// DeepCopyData crea una copia profunda solo del campo Data
func (n *Node) DeepCopyData() map[string]interface{} {
	if n.Data == nil {
		return nil
	}
	
	// Crear un nuevo mapa
	dataCopy := make(map[string]interface{})
	
	// Copiar cada elemento
	for k, v := range n.Data {
		// Para valores complejos, usar JSON para copiar
		if complexValue, needsCopy := needsDeepCopy(v); needsCopy {
			dataCopy[k] = complexValue
		} else {
			// Para tipos simples, asignación directa es suficiente
			dataCopy[k] = v
		}
	}
	
	return dataCopy
}

// needsDeepCopy verifica si un valor necesita copia profunda
func needsDeepCopy(v interface{}) (interface{}, bool) {
	switch val := v.(type) {
	case map[string]interface{}, []interface{}:
		// Estos tipos necesitan copia profunda
		data, _ := json.Marshal(val)
		var copy interface{}
		json.Unmarshal(data, &copy)
		return copy, true
	default:
		// Tipos primitivos no necesitan copia profunda
		return val, false
	}
}