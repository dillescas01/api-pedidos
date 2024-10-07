# Usar una imagen oficial de Go
FROM golang:1.20-alpine

# Establecer directorio de trabajo
WORKDIR /programas

# Copiar go.mod y go.sum e instalar dependencias
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
