# Documentación de Rate Limiting

## Descripción General

nFlow Runtime incluye un sistema de limitación de tasa (rate limiting) configurable basado en IP para proteger tu API del abuso y garantizar un uso justo. El limitador de tasa utiliza un algoritmo de token bucket y soporta backends tanto en memoria como Redis para implementaciones distribuidas.

## Características

- **Limitación basada en IP**: Limita las solicitudes por dirección IP
- **Algoritmo token bucket**: Permite tráfico en ráfagas mientras mantiene la tasa promedio
- **Múltiples backends**: En memoria (instancia única) o Redis (distribuido)
- **Exclusión de rutas**: Exime rutas específicas como health checks
- **Exclusión de IPs**: Lista blanca de IPs o rangos de IP confiables
- **Respuestas configurables**: Mensajes de error personalizados y headers Retry-After
- **Cero impacto cuando está deshabilitado**: Sin sobrecarga cuando el rate limiting está apagado

## Configuración

El rate limiting se configura en el archivo `config.toml`:

```toml
[rate_limit]
enabled = false                   # Habilitar rate limiting (default: false)

# Configuración de rate limiting por IP
ip_rate_limit = 100              # Solicitudes por IP por ventana
ip_window_minutes = 1            # Ventana de tiempo en minutos
ip_burst_size = 10               # Tamaño de ráfaga para limiting por IP

# Backend de almacenamiento
backend = "memory"               # "memory" o "redis"
cleanup_interval = 10            # Intervalo de limpieza en minutos (backend memory)

# Configuración de respuesta
retry_after_header = true        # Incluir header Retry-After
error_message = "Límite de tasa excedido. Por favor intente más tarde."

# Exclusiones (separadas por comas)
excluded_ips = ""                # ej., "127.0.0.1,192.168.1.0/24"
excluded_paths = "/health,/metrics"  # Rutas a excluir del rate limiting
```

### Parámetros de Configuración

#### Configuración Básica

- **`enabled`**: Interruptor principal para rate limiting. Establecer en `true` para habilitar.
- **`ip_rate_limit`**: Número máximo de solicitudes permitidas por IP en la ventana de tiempo
- **`ip_window_minutes`**: Duración de la ventana de tiempo en minutos
- **`ip_burst_size`**: Solicitudes adicionales permitidas para manejar ráfagas de tráfico

#### Selección de Backend

- **`backend`**: Elegir backend de almacenamiento
  - `"memory"`: Usa almacenamiento en memoria (default). Mejor para implementaciones de instancia única.
  - `"redis"`: Usa Redis para rate limiting distribuido entre múltiples instancias.
- **`cleanup_interval`**: Para backend de memoria, con qué frecuencia limpiar entradas expiradas (en minutos)

#### Configuración de Respuesta

- **`retry_after_header`**: Cuando es `true`, incluye el header `Retry-After` en respuestas 429
- **`error_message`**: Mensaje de error personalizado devuelto cuando se excede el límite

#### Exclusiones

- **`excluded_ips`**: Lista separada por comas de IPs o rangos CIDR a excluir del rate limiting
  - Ejemplos: `"127.0.0.1"`, `"192.168.1.0/24"`, `"10.0.0.0/8,172.16.0.0/12"`
- **`excluded_paths`**: Lista separada por comas de prefijos de ruta a excluir
  - Ejemplos: `"/health"`, `"/metrics"`, `"/health,/metrics,/api/public"`

## Cómo Funciona

### Algoritmo Token Bucket

El limitador de tasa usa un algoritmo token bucket:

1. Cada dirección IP obtiene un bucket con `ip_rate_limit` tokens
2. Cada solicitud consume un token
3. Los tokens se rellenan a una tasa de `ip_rate_limit` por `ip_window_minutes`
4. El bucket puede contener hasta `ip_rate_limit + ip_burst_size` tokens
5. Si no hay tokens disponibles, la solicitud es rechazada con HTTP 429

### Escenarios de Ejemplo

**Configuración:**
```toml
ip_rate_limit = 60
ip_window_minutes = 1
ip_burst_size = 10
```

Esto permite:
- 60 solicitudes por minuto en promedio
- Hasta 70 solicitudes en una ráfaga (60 + 10)
- Después de una ráfaga, el cliente debe esperar a que se rellenen los tokens

## Headers de Respuesta

Cuando el rate limiting está activo, se incluyen los siguientes headers:

### Solicitudes Exitosas
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1672531260
```

### Solicitudes Limitadas (HTTP 429)
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1672531260
Retry-After: 30
```

Cuerpo de respuesta:
```json
{
  "error": "Límite de tasa excedido. Por favor intente más tarde.",
  "retry_after": 30
}
```

## Comparación de Backends

### Backend de Memoria

**Ventajas:**
- Configuración cero
- Muy rápido (latencia en microsegundos)
- Sin dependencias externas

**Desventajas:**
- No apto para implementaciones distribuidas
- Los límites son por instancia, no globales
- Datos perdidos al reiniciar

**Usar cuando:**
- Ejecutas una sola instancia
- No necesitas datos persistentes de límite de tasa
- Quieres máximo rendimiento

### Backend Redis

**Ventajas:**
- Funciona entre múltiples instancias
- Datos de límite de tasa persistentes
- Verdadero rate limiting distribuido

**Desventajas:**
- Requiere configuración de Redis
- Latencia ligeramente mayor (milisegundos)
- Dependencia de infraestructura adicional

**Usar cuando:**
- Ejecutas múltiples instancias
- Necesitas rate limiting consistente entre todas las instancias
- Ya usas Redis para sesiones

## Ejemplos

### Configuración Básica (Instancia Única)

```toml
[rate_limit]
enabled = true
ip_rate_limit = 100
ip_window_minutes = 1
backend = "memory"
```

### Configuración de Producción (Múltiples Instancias)

```toml
[rate_limit]
enabled = true
ip_rate_limit = 1000
ip_window_minutes = 1
ip_burst_size = 50
backend = "redis"
excluded_ips = "10.0.0.0/8"  # Red interna
excluded_paths = "/health,/metrics"
```

### Protección API Estricta

```toml
[rate_limit]
enabled = true
ip_rate_limit = 10
ip_window_minutes = 1
ip_burst_size = 0  # Sin ráfagas permitidas
retry_after_header = true
error_message = "Límite de API excedido. Máximo 10 solicitudes por minuto."
```

## Detección de IP del Cliente

El limitador de tasa detecta las IPs del cliente en el siguiente orden:

1. Header `X-Real-IP` (establecido por proxies inversos)
2. Header `X-Forwarded-For` (IP más a la izquierda si hay múltiples)
3. `RemoteAddr` de la conexión

### Detrás de un Proxy Inverso

Asegúrate de que tu proxy inverso establezca los headers apropiados:

**Nginx:**
```nginx
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

**Apache:**
```apache
RequestHeader set X-Real-IP "%{REMOTE_ADDR}s"
RequestHeader set X-Forwarded-For "%{REMOTE_ADDR}s"
```

## Monitoreo

### Logs

Cuando el logging detallado está habilitado (flag `-v`), se registran eventos de rate limit:

```
Límite de tasa excedido para IP: 192.168.1.100, ruta: /api/workflow
```

### Métricas

Si las métricas Prometheus están habilitadas, puedes monitorear:
- Total de solicitudes limitadas por tasa
- Hits de límite de tasa por IP
- Estados actuales de buckets

## Solución de Problemas

### El Rate Limiting No Funciona

1. Verifica `enabled = true` en config.toml
2. Verifica que la configuración se cargue (revisa logs de inicio)
3. Asegúrate de no estar accediendo a rutas o IPs excluidas

### Todas las Solicitudes Están Siendo Limitadas

1. Verifica si `ip_rate_limit` es muy bajo
2. Verifica que las ventanas de tiempo sean apropiadas
3. Revisa la detección de IP del cliente (podría estar viendo IP del proxy)

### Problemas con Backend Redis

1. Verifica la conexión Redis en los logs
2. Comprueba que Redis sea accesible desde la aplicación
3. Asegúrate de que Redis tenga suficiente memoria para las claves de rate limit

## Mejores Prácticas

1. **Empieza Conservador**: Comienza con límites más altos y reduce según sea necesario
2. **Monitorea el Impacto**: Observa tus métricas después de habilitar
3. **Excluye Health Checks**: Siempre excluye endpoints de monitoreo
4. **Usa Burst para APIs**: Permite algo de ráfaga para manejar picos legítimos de tráfico
5. **Límites Diferentes para Diferentes Rutas**: Considera usar un proxy inverso para límites específicos por ruta

## Consideraciones de Seguridad

1. **IP Spoofing**: En producción, asegura la validación apropiada de headers en tu proxy edge
2. **Ataques Distribuidos**: Considera protección DDoS adicional a nivel de red
3. **Agotamiento de Recursos**: Monitorea el uso de memoria con backend de memoria bajo condiciones de ataque

## Impacto en el Rendimiento

- **Deshabilitado**: Cero sobrecarga
- **Backend Memoria**: ~1-2 microsegundos por solicitud
- **Backend Redis**: ~1-5 milisegundos por solicitud (depende de la latencia de Redis)

El limitador de tasa está diseñado para tener un impacto mínimo en el tráfico legítimo mientras previene efectivamente el abuso.