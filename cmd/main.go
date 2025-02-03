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
	ID          int     `json:"id"`
	GraphID     int     `json:"graph_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	X           float32 `json:"x"`
	Y           float32 `json:"y"`
}

type Relation struct {
	ID          int    `json:"id"`
	GraphID     int    `json:"graph_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	FromService int    `json:"from_service"`
	ToService   int    `json:"to_service"`
}

type Graph struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	MaxNodeID int        `json:"max_node_id"`
	MaxEdgeID int        `json:"max_edge_id"`
	Services  []Service  `json:"services"`
	Relations []Relation `json:"relations"`
}

var db *sql.DB

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
		SELECT jsonb_build_object(
			'id', g.id,
			'name', g.name,
			'max_node_id', g.max_node_id,
			'max_edge_id',  g.max_edge_id,
			'services', COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', s.id,
					'name', s.name,
					'description', s.description,
					'x', s.x,
					'y', s.y
				)) FROM services s WHERE s.graph_id = g.id
			), '[]'::jsonb),
			'relations', COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', r.id,
					'name', r.name,
					'description', r.description,
					'from_service', r.from_service,
					'to_service', r.to_service
				)) FROM relations r WHERE r.graph_id = g.id
			), '[]'::jsonb)
		) AS graph
		FROM graphs g
		WHERE g.id = $1;

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
	query := `INSERT INTO services (id, graph_id, name, description, x, y) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := db.Exec(query, service.ID, service.GraphID, service.Name, service.Description, service.X, service.Y)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		return
	}

	query = `UPDATE graphs SET max_node_id = GREATEST(max_node_id, $1) WHERE id = $2`
	_, err = db.Exec(query, service.ID+1, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed update max id"})
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
	query = `UPDATE graphs SET max_edge_id = GREATEST(max_edge_id, $1) WHERE id = $2`
	_, err = db.Exec(query, relation.ID+1, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed update max id"})
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

	query := `UPDATE services SET name = $1, description = $2, x = $3, y = $4 WHERE id = $5`
	_, err := db.Exec(query, service.Name, service.Description, service.X, service.Y, serviceID)
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
