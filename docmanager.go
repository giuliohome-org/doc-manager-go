package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type Document struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var (
	accountName   = os.Getenv("AZURE_STORAGE_ACCOUNT")
	accountKey    = os.Getenv("AZURE_STORAGE_KEY")
	containerName = os.Getenv("AZURE_STORAGE_CONTAINER")
)

func getBlobServiceClient() (*azblob.Client, error) {
	url := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}
	client, err := azblob.NewClientWithSharedKeyCredential(url, cred, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func createDocument(c *gin.Context) {
	var doc Document
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	doc.ID = uuid.New().String()

	client, err := getBlobServiceClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Azure client"})
		return
	}
	
	blobName := doc.ID
	blobData := []byte(fmt.Sprintf(`{"id":"%s", "name":"%s"}`, doc.ID, doc.Name))
	_, err = client.UploadBuffer(context.TODO(), containerName, blobName, blobData, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload document"})
		return
	}

	c.JSON(http.StatusOK, doc)
}

func getDocument(c *gin.Context) {
	id := c.Param("id")
	client, err := getBlobServiceClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Azure client"})
		return
	}
	
	blobName := id
	resp, err := client.DownloadStream(context.TODO(), containerName, blobName, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	defer resp.Body.Close()

	// Read the response body into a byte slice
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read document"})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

func listDocuments(c *gin.Context) {
	client, err := getBlobServiceClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Azure client"})
		return
	}

	pager := client.NewListBlobsFlatPager(containerName, nil)
	var docs []Document

	for pager.More() {
		resp, err := pager.NextPage(context.TODO())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list documents"})
			return
		}

		for _, blob := range resp.Segment.BlobItems {
			docs = append(docs, Document{
				ID:   *blob.Name,
				Name: *blob.Name, // Adjust if needed based on how names are stored
			})
		}
	}

	c.JSON(http.StatusOK, docs)
}

func main() {
	if accountName == "" || accountKey == "" || containerName == "" {
		log.Fatal("Azure Storage credentials are not set")
	}

	r := gin.Default()

	r.GET("/documents", listDocuments)

	r.POST("/document", createDocument)
	r.GET("/document/:id", getDocument)

	log.Println("Starting server on :8080")
	r.Run(":8080")
}
