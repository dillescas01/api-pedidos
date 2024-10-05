package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "io/ioutil"
    _ "github.com/lib/pq"
)

type Pedido struct {
    Cliente  string        `json:"cliente"`
    Detalles []DetallePedido `json:"detalles"`
}

type DetallePedido struct {
    ProductoID    int     `json:"producto_id"`
    Cantidad      int     `json:"cantidad"`
    PrecioUnitario float64 `json:"precio_unitario"`
}

type Producto struct {
    ID        int     `json:"id"`
    Nombre    string  `json:"nombre"`
    Inventario int    `json:"inventario"`
    Precio    float64 `json:"precio"`
}

func main() {
    http.HandleFunc("/pedidos", crearPedido)
    log.Fatal(http.ListenAndServe(":8080", nil))
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
    connStr := "user=tu_usuario dbname=bd_api_pedidos password=tu_contraseña host=localhost sslmode=disable"
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Verificar el inventario de cada producto llamando al microservicio de productos
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
    url := fmt.Sprintf("http://productos:5000/productos/%d", productoID)
    resp, err := http.Get(url)
    if err != nil {
        return Producto{}, err
    }
    defer resp.Body.Close()

    var producto Producto
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return Producto{}, err
    }

    err = json.Unmarshal(body, &producto)
    if err != nil {
        return Producto{}, err
    }

    return producto, nil
}

// Función para actualizar el inventario del producto en el microservicio de productos
func actualizarInventario(productoID, cantidad int) error {
    url := fmt.Sprintf("http://productos:5000/productos/%d/actualizar_inventario", productoID)
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
