# Usar una imagen oficial de Go
FROM golang:1.20-alpine

# Establecer el directorio de trabajo dentro del contenedor
WORKDIR /programas

# Copiar go.mod e instalar dependencias (genera go.sum automáticamente)
COPY go.mod ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar la aplicación
RUN go build -o api-pedidos

# Exponer el puerto 8001
EXPOSE 8001

# Comando para ejecutar la aplicación
CMD ["/programas/api-pedidos"]
