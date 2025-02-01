package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hse-telescope/utils/db/psql"
	_ "github.com/lib/pq"
)

type Service struct {
	ID          string `json:"id"`
	GraphID     string `jsom:"graph_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Relation struct {
	ID          string `json:"id"`
	GraphID     string `jsom:"graph_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	FromService string `json:"from_service"`
	ToService   string `json:"to_service"`
}

type Graph struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var db *sql.DB

const (
	host     = "mvp-db"
	port     = 5432
	user     = "user"
	password = "password"
	dbname   = "graphs"
)

func setupDB() {
	dbConf := psql.DB{
		Schema:         psql.PGDriver,
		User:           "user",
		Password:       "password",
		IP:             "mvp-db",
		Port:           5432,
		DataBase:       "graphs",
		SSL:            "disable",
		MigrationsPath: "file://migrations",
	}

	var err error
	db, err = sql.Open(psql.PGDriver, dbConf.GetDBURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}

	err = psql.MigrateDB(
		db,
		dbConf.MigrationsPath,
		dbConf.Schema,
	)
	if err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}
}

func main() {
	setupDB()

	r := gin.Default()

	r.GET("/api/v1/ping", ping)

	r.GET("/api/v1/graph/:id", getGraphByID)

	r.POST("/api/v1/services", createService)
	r.POST("/api/v1/relations", createRelation)

	r.GET("/api/v1/services/:id", getServiceById)
	r.GET("/api/v1/relations/:id", getRelationById)

	r.PUT("/api/v1/services/:id", updateService)
	r.PUT("/api/v1/relations/:id", updateRelation)

	r.DELETE("/api/v1/services/:id", deleteService)
	r.DELETE("/api/v1/relations/:id", deleteRelation)

	r.Run(":8080")
}

func ping(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, map[string]string{"response": "pong"})
}

func getGraphByID(c *gin.Context) {
	graphID := c.Param("id")

	query := `
		SELECT json_build_object(
			'services', json_agg(
				json_build_object(
					'id', s.id,
					'name', s.name,
					'description', s.description
				)
			),
			'relations', json_agg(
				json_build_object(
					'id', r.id,
					'name', r.name,
					'description', r.description,
					'from_service', sf.name,
					'to_service', st.name
				)
			)
		) AS graph
		FROM services s
		LEFT JOIN relations r
			ON r.graph_id = s.graph_id
		LEFT JOIN services sf
			ON r.from_service_id = sf.id
		LEFT JOIN services st
			ON r.to_service_id = st.id
		WHERE s.graph_id = $1
		GROUP BY s.graph_id;
	`

	var graphJSON []byte
	err := db.QueryRow(query, graphID).Scan(&graphJSON)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve graph"})
		return
	}

	var graph Graph
	err = json.Unmarshal(graphJSON, &graph)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse graph JSON"})
		return
	}

	c.JSON(http.StatusOK, graph)
}

func createService(c *gin.Context) {
	var service Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := `INSERT INTO services (id, graph_id, name, description) VALUES ($1, $2, $3, $4)`
	_, err := db.Exec(query, service.ID, service.GraphID, service.Name, service.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Service created successfully"})
}

func createRelation(c *gin.Context) {
	var relation Relation
	if err := c.ShouldBindJSON(&relation); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := `INSERT INTO relations (id, graph_id, name, description, from_service, to_service) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := db.Exec(query, relation.ID, relation.GraphID, relation.Name, relation.Description, relation.FromService, relation.ToService)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create relation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Relation created successfully"})
}

func updateService(c *gin.Context) {
	serviceID := c.Param("id")
	var service Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := `UPDATE services SET name = $1, description = $2 WHERE id = $3`
	_, err := db.Exec(query, service.Name, service.Description, serviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Service updated successfully"})
}

func updateRelation(c *gin.Context) {
	relationID := c.Param("id")
	var relation Relation
	if err := c.ShouldBindJSON(&relation); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := `UPDATE relations SET name = $1, description = $2, from_service = $3, to_service = $4 WHERE id = $5`
	_, err := db.Exec(query, relation.Name, relation.Description, relation.FromService, relation.ToService, relationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update relation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Relation updated successfully"})
}

func deleteService(c *gin.Context) {
	serviceID := c.Param("id")

	query := `DELETE FROM services WHERE id = $1`
	_, err := db.Exec(query, serviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Service deleted successfully"})
}

func deleteRelation(c *gin.Context) {
	relationID := c.Param("id")

	query := `DELETE FROM relations WHERE id = $1`
	_, err := db.Exec(query, relationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete relation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Relation deleted successfully"})
}

func getServiceById(c *gin.Context) {
	serviceID := c.Param("service-id")

	var service Service
	err := db.QueryRow("SELECT id, graph_id, name, description FROM services WHERE id = $1", serviceID).
		Scan(&service.ID, &service.GraphID, &service.Name, &service.Description)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	c.JSON(http.StatusOK, service)
}

func getRelationById(c *gin.Context) {
	relationID := c.Param("relation-id")

	var relation Relation
	err := db.QueryRow("SELECT id, graph_id, name, description, from_service, to_service FROM relations WHERE id = $1", relationID).
		Scan(&relation.ID, &relation.GraphID, &relation.Description, &relation.Description, &relation.FromService, &relation.ToService)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Relation not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	c.JSON(http.StatusOK, relation)
}
