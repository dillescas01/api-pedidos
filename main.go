package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"io/ioutil"
	"strconv"
	"github.com/gorilla/mux"  // Se añade mux para manejar parámetros en las rutas
	_ "github.com/lib/pq"
)

// Estructura de Pedido
type Pedido struct {
	ID       int            `json:"id_pedido"`
	Cliente  string         `json:"cliente"`
	Fecha    string         `json:"fecha"`
	Estado   string         `json:"estado"`
	Detalles []DetallePedido `json:"detalles"`
}

// Estructura de DetallePedido
type DetallePedido struct {
	ProductoID    int     `json:"producto_id"`
	Cantidad      int     `json:"cantidad"`
	PrecioUnitario float64 `json:"precio_unitario"`
}

// Estructura de Producto
type Producto struct {
	ID        int     `json:"id"`
	Nombre    string  `json:"nombre"`
	Inventario int    `json:"inventario"`
	Precio    float64 `json:"precio"`
}

// Estructura para la respuesta del echo test
type Message struct {
	Message string `json:"message"`
}

func main() {
	// Configurar enrutador
	r := mux.NewRouter()

	// Rutas
	r.HandleFunc("/", getEchoTest).Methods("GET")
	r.HandleFunc("/pedidos", crearPedido).Methods("POST")
	r.HandleFunc("/pedidos", obtenerTodosPedidos).Methods("GET")
	r.HandleFunc("/pedidos/{id}", obtenerPedido).Methods("GET")

	// Levantar el servidor en el puerto 8001
	log.Fatal(http.ListenAndServe(":8001", r))
}

// Función para el echo test (health check)
func getEchoTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := Message{Message: "Echo Test OK"}
	json.NewEncoder(w).Encode(response)
}

// Función para crear pedidos
func crearPedido(w http.ResponseWriter, r *http.Request) {
	var pedido Pedido
	err := json.NewDecoder(r.Body).Decode(&pedido)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Conectar a la base de datos PostgreSQL
	connStr := "user=postgres dbname=bd_api_pedidos password=utec host=98.82.74.138 sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Verificar el inventario de cada producto llamando al microservicio de productos en el puerto 8000
	for _, detalle := range pedido.Detalles {
		producto, err := obtenerProducto(detalle.ProductoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if producto.Inventario < detalle.Cantidad {
			http.Error(w, fmt.Sprintf("Inventario insuficiente para el producto ID %d", detalle.ProductoID), http.StatusBadRequest)
			return
		}
	}

	// Crear pedido en la base de datos
	var idPedido int
	err = db.QueryRow("INSERT INTO pedidos (cliente, fecha, estado) VALUES ($1, CURRENT_DATE, 'Pendiente') RETURNING id_pedido", pedido.Cliente).Scan(&idPedido)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Insertar los detalles del pedido
	for _, detalle := range pedido.Detalles {
		_, err := db.Exec("INSERT INTO detalle_pedido (id_pedido, producto_id, cantidad, precio_unitario) VALUES ($1, $2, $3, $4)",
			idPedido, detalle.ProductoID, detalle.Cantidad, detalle.PrecioUnitario)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Actualizar el inventario en el microservicio de productos
		actualizarInventario(detalle.ProductoID, detalle.Cantidad)
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Pedido creado con ID: %d", idPedido)
}

// Función para obtener detalles del producto desde el microservicio de productos
func obtenerProducto(productoID int) (Producto, error) {
	url := fmt.Sprintf("http://productos:8000/productos/%d", productoID)
	resp, err := http.Get(url)
	if err != nil {
		return Producto{}, err
	}
	defer resp.Body.Close()

	var productoResponse struct {
		Producto Producto `json:"producto"`
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Producto{}, err
	}

	err = json.Unmarshal(body, &productoResponse)
	if err != nil {
		return Producto{}, err
	}

	return productoResponse.Producto, nil
}

// Función para actualizar el inventario del producto en el microservicio de productos
func actualizarInventario(productoID, cantidad int) error {
	url := fmt.Sprintf("http://productos:8000/productos/%d/actualizar_inventario", productoID)
	reqBody, _ := json.Marshal(map[string]int{
		"cantidad": cantidad,
	})

	resp, err := http.Post(url, "application/json", ioutil.NopCloser(bytes.NewReader(reqBody)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error actualizando el inventario del producto ID %d", productoID)
	}

	return nil
}

// Nuevo endpoint para obtener un pedido por ID
func obtenerPedido(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idPedido, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "ID de pedido inválido", http.StatusBadRequest)
		return
	}

	connStr := "user=postgres dbname=bd_api_pedidos password=utec host=98.82.74.138 sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		http.Error(w, "Error al conectar con la base de datos", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var pedido Pedido
	err = db.QueryRow("SELECT id_pedido, cliente, fecha, estado FROM pedidos WHERE id_pedido = $1", idPedido).Scan(&pedido.ID, &pedido.Cliente, &pedido.Fecha, &pedido.Estado)
	if err != nil {
		http.Error(w, "Pedido no encontrado", http.StatusNotFound)
		return
	}

	rows, err := db.Query("SELECT producto_id, cantidad, precio_unitario FROM detalle_pedido WHERE id_pedido = $1", idPedido)
	if err != nil {
		http.Error(w, "Error al obtener detalles del pedido", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var detalle DetallePedido
		if err := rows.Scan(&detalle.ProductoID, &detalle.Cantidad, &detalle.PrecioUnitario); err != nil {
			http.Error(w, "Error al leer detalle del pedido", http.StatusInternalServerError)
			return
		}
		pedido.Detalles = append(pedido.Detalles, detalle)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pedido)
}

// Nuevo endpoint para obtener todos los pedidos
func obtenerTodosPedidos(w http.ResponseWriter, r *http.Request) {
	connStr := "user=postgres dbname=bd_api_pedidos password=utec host=98.82.74.138 sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		http.Error(w, "Error al conectar con la base de datos", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id_pedido, cliente, fecha, estado FROM pedidos")
	if err != nil {
		http.Error(w, "Error al obtener pedidos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pedidos []Pedido
	for rows.Next() {
		var pedido Pedido
		err := rows.Scan(&pedido.ID, &pedido.Cliente, &pedido.Fecha, &pedido.Estado)
		if err != nil {
			http.Error(w, "Error al leer pedido", http.StatusInternalServerError)
			return
		}

		// Obtener detalles del pedido
		detalleRows, err := db.Query("SELECT producto_id, cantidad, precio_unitario FROM detalle_pedido WHERE id_pedido = $1", pedido.ID)
		if err != nil {
			http.Error(w, "Error al obtener detalles del pedido", http.StatusInternalServerError)
			return
		}
		defer detalleRows.Close()

		for detalleRows.Next() {
			var detalle DetallePedido
			if err := detalleRows.Scan(&detalle.ProductoID, &detalle.Cantidad, &detalle.PrecioUnitario); err != nil {
				http.Error(w, "Error al leer detalle del pedido", http.StatusInternalServerError)
				return
			}
			pedido.Detalles = append(pedido.Detalles, detalle)
		}

		pedidos = append(pedidos, pedido)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pedidos)
}
