package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "go.uber.org/automaxprocs"
)

type Transaction struct {
	Type        string `json:"tipo"`
	Description string `json:"descricao"`
	Amount      int    `json:"valor"`
}

type StatementDetails struct {
	CreatedAt   time.Time `json:"realizada_em" db:"created_at"`
	Type        string    `json:"tipo" db:"type"`
	Description string    `json:"descricao" db:"description"`
	Value       int       `json:"valor" db:"value"`
}

type BalanceDetails struct {
	Date  time.Time `json:"data_extrato"`
	Total int       `json:"total"`
	Limit int       `json:"limite"`
}

type Statement struct {
	Statements []StatementDetails `json:"ultimas_transacoes"`
	Balance    BalanceDetails     `json:"saldo"`
}

type User struct {
	Id      int `json:"-" db:"id"`
	Limit   int `json:"limite" db:"limit"`
	Balance int `json:"total" db:"balance"`
}

func main() {

	pool, err := NewPool(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/clientes/:id/transacoes", transactionHandler(pool))
	router.GET("/clientes/:id/extrato", statementHandler(pool))

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func statementHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		id := c.Param("id")

		if id == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "missing user id"})
			return
		}

		// Get a conn from pool
		conn, err := pool.Acquire(ctx)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		defer conn.Release()

		tx, err := conn.Begin(ctx)

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		rows, err := tx.Query(ctx, `SELECT u.id, u."limit", u.balance FROM users u WHERE u.id = $1;`, id)

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[User])

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				tx.Rollback(ctx)
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Get transactions from user

		rows, err = tx.Query(ctx, `SELECT t.value, t.type, t.description, t.created_at FROM transactions t WHERE t.user_id = $1 ORDER BY t.created_at DESC LIMIT 10;`, user.Id)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				tx.Rollback(ctx)
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[StatementDetails])

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		tx.Commit(ctx)

		c.JSON(http.StatusOK, Statement{
			Balance: BalanceDetails{
				Total: user.Balance,
				Date:  time.Now(),
				Limit: user.Limit,
			},
			Statements: transactions,
		})

	}
}

func transactionHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		id := c.Param("id")

		if id == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "missing user id"})
			return
		}

		var transaction Transaction

		err := c.Bind(&transaction)

		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid payload"})
			return
		}

		if transaction.Type != "d" && transaction.Type != "c" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid transaction type"})
			return
		}

		if len(transaction.Description) > 10 || len(transaction.Description) < 1 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid transaction description"})
			return
		}

		// Get a conn from pool
		conn, err := pool.Acquire(ctx)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		defer conn.Release()

		// Get User information

		tx, err := conn.Begin(ctx)

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		rows, err := tx.Query(ctx, `SELECT u.id, u."limit", u.balance FROM users u WHERE u.id = $1 FOR UPDATE;`, id)

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[User])

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				tx.Rollback(ctx)
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Process transaction

		var balance int

		switch transaction.Type {
		case "c":
			balance += user.Balance + transaction.Amount
		case "d":
			balance = user.Balance - transaction.Amount
		}

		if balance < (-user.Limit) {
			tx.Rollback(ctx)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no limit"})
			return
		}

		_, err = tx.Exec(ctx, "INSERT INTO transactions (value, type, description, user_id) VALUES ($1, $2, $3, $4);", transaction.Amount, transaction.Type, transaction.Description, id)

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_, err = tx.Exec(ctx, "UPDATE users SET balance = $1 WHERE id = $2;", balance, id)

		if err != nil {
			tx.Rollback(ctx)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		err = tx.Commit(ctx)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"saldo":  balance,
			"limite": user.Limit,
		})
	}
}

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(os.Getenv("DNS"))

	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)

	if err != nil {
		return nil, err
	}

	return pool, err
}
