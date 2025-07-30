package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/cbroglie/mustache"
	"github.com/labstack/echo/v4"
)

type IAnFlow string

var (
	fxsIA map[string]interface{} = make(map[string]interface{})
)

func (d IAnFlow) Run(c echo.Context,
	vars map[string]string, payloadIn interface{}, dromedaryData string,
	callback chan string,
) (payloadOut interface{}, next string, err error) {
	return nil, "output_1", nil
}

func init() {
	addFeatureIANflow()
}

func (d IAnFlow) AddFeatureJS() map[string]interface{} {
	return fxsIA
}

func (d IAnFlow) Name() string {
	return "ia"
}

// --- Estructuras para la solicitud a Gemini ---
type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// --- Estructuras para la respuesta de Gemini ---
type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

// gemini realiza una llamada a la API de Gemini con el prompt y devuelve la respuesta.
// El modelo por defecto es "gemini-1.5-pro", pero se puede especificar otro modelo.
// El parámetro data se utiliza para renderizar el prompt con mustache.
// Si ocurre un error, se devuelve un mensaje de error.
func gemini(apiKey string, prompt string, model string, data interface{}) (string, error) {
	// La URL de Gemini incluye el modelo y la clave de API como parámetro.
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	// Renderiza el prompt con Mustache, igual que en tu función original.
	promptFinal, err := mustache.Render(prompt, data)
	if err != nil {
		log.Println("Error renderizando con Mustache:", err)
		promptFinal = prompt // Usa el prompt original si falla la renderización
	}

	log.Println("Gemini prompt:", promptFinal)

	// Estructura del cuerpo de la solicitud para Gemini.
	requestBody, err := json.Marshal(GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: promptFinal},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("error al codificar el cuerpo de la solicitud: %w", err)
	}

	// Creación de la solicitud HTTP POST.
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error al crear la solicitud HTTP: %w", err)
	}

	// Encabezado necesario para la solicitud.
	req.Header.Set("Content-Type", "application/json")

	// Ejecución de la solicitud.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error al realizar la solicitud a Gemini: %w", err)
	}
	defer resp.Body.Close()

	// Verificación del código de estado de la respuesta.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error en la respuesta de Gemini: status %d - %s", resp.StatusCode, string(body))
	}

	// Decodificación de la respuesta JSON de Gemini.
	var result GeminiResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("error al decodificar la respuesta JSON: %w", err)
	}

	// Extracción del texto de la respuesta.
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("respuesta de Gemini vacía o con formato inesperado")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

type OllamaMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // contenido del mensaje
}

type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaChatResponse struct {
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func ollama(baseURL string, system string, prompt string, model string, data interface{}) (string, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	endpoint, err := url.JoinPath(baseURL, "/api/chat")
	if err != nil {
		return "", fmt.Errorf("URL base inválida: %w", err)
	}

	// Render prompt y system con Mustache
	promptFinal, err := mustache.Render(prompt, data)
	if err != nil {
		log.Println("Error renderizando prompt con Mustache:", err)
		promptFinal = prompt
	}
	systemFinal := ""
	if system != "" {
		systemFinal, err = mustache.Render(system, data)
		if err != nil {
			log.Println("Error renderizando system con Mustache:", err)
			systemFinal = system
		}
	}

	log.Println("Ollama system:", systemFinal)
	log.Println("Ollama user prompt:", promptFinal)

	// Armar la lista de mensajes
	var messages []OllamaMessage
	if systemFinal != "" {
		messages = append(messages, OllamaMessage{
			Role:    "system",
			Content: systemFinal,
		})
	}
	messages = append(messages, OllamaMessage{
		Role:    "user",
		Content: promptFinal,
	})

	// Armar el cuerpo de la solicitud
	body, err := json.Marshal(OllamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("error serializando JSON: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("error creando solicitud HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error conectando con Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("respuesta con error: %s", string(body))
	}

	var result OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return result.Message.Content, nil
}

// openAI realiza una llamada a la API de OpenAI con el prompt y devuelve la respuesta.
// El modelo por defecto es "gpt-3.5-turbo", pero se puede especificar otro modelo.
// El parámetro data se utiliza para renderizar el prompt con mustache.
// Si ocurre un error, se devuelve un mensaje de error.
func openAI(apiKey string, system string, prompt string, model string, data interface{}) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	// Construcción del mensaje

	messages := []map[string]string{}
	if system != "" {
		system_final, err := mustache.Render(system, data)
		if err != nil {
			log.Println(err)
			system_final = system
		}
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": system_final,
		})
		log.Println("OpenAI system message:", system_final)
	}

	prompt_final, err := mustache.Render(prompt, data)
	if err != nil {
		log.Println(err)
		prompt_final = prompt
	}
	log.Println("OpenAI prompt:", prompt_final)
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt_final,
	})

	// Estructura del request
	requestBody, err := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": messages,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	// Encabezados
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error: status %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}

func addFeatureIANflow() {
	fxsIA["openai"] = openAI
	fxsIA["ollama"] = ollama
	fxsIA["gemini"] = gemini
}
