package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Document struct {
	ID          string  `json:"id"`
	Description string  `json:"description,omitempty"`
	IsFile      bool    `json:"is_file,omitempty"`
	Content     string  `json:"content"`
	FileID      *string `json:"file_id,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
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

type DocumentForm struct {
	Content string                `form:"content" binding:"required"`
	File    *multipart.FileHeader `form:"file"`
}

func createDocument(c *gin.Context) {
	var form DocumentForm
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Invalid form data"})
		return
	}

	id := uuid.New().String()
	log.Println("Creating document", id)
	content := []byte(form.Content)
	client, err := getBlobServiceClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Azure client"})
		return
	}

	containerClient := client.ServiceClient().NewContainerClient("documents")
	blobClient := containerClient.NewBlockBlobClient(id)
	_, err = blobClient.UploadStream(context.TODO(), io.NopCloser(bytes.NewReader(content)), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: fmt.Sprintf("Failed to create document: %v", err)})
		return
	}

	var fileID *string
	file, err := c.FormFile("file")
	if err == nil {
		fileName := fmt.Sprintf("%s_%s", id, file.Filename)
		fileClient := containerClient.NewBlockBlobClient(fileName)
		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Message: fmt.Sprintf("Failed to open file: %v", err)})
			return
		}
		defer f.Close()

		_, err = fileClient.UploadStream(context.TODO(), f, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Message: fmt.Sprintf("Failed to upload file: %v", err)})
			return
		}

		fileID = &fileName
	}

	c.JSON(http.StatusOK, Document{
		ID:      id,
		Content: form.Content,
		FileID:  fileID,
	})
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

	c.JSON(http.StatusOK, Document{
		ID:      id,
		Content: string(data),
	})
}

func downloadDocument(c *gin.Context) {
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

	c.Data(http.StatusOK, "application/octet-stream", data)
}

func azListDocs() ([]Document, string) {
	client, err := getBlobServiceClient()
	if err != nil {

		return nil, "Failed to create Azure client"
	}

	pager := client.NewListBlobsFlatPager(containerName, nil)
	var docs []Document

	for pager.More() {
		resp, err := pager.NextPage(context.TODO())
		if err != nil {
			return nil, "Failed to list documents"
		}

		for _, blob := range resp.Segment.BlobItems {
			content := ""
			IsFile := false
			if blob.Name != nil {
				id := *blob.Name
				if !strings.Contains(*blob.Name, "_") {
					log.Println("Blob name:", *blob.Name)
					blobResp, err := client.DownloadStream(context.TODO(), containerName, *blob.Name, nil)
					if err != nil {
						return nil, "Failed to download blob content"
					}
					defer blobResp.Body.Close()

					data, err := io.ReadAll(blobResp.Body)
					if err != nil {
						return nil, "Failed to read blob content"
					}
					content = string(data)
				} else {
					parts := strings.SplitN(*blob.Name, "_", 2)
					content = parts[1]
					id = parts[0]
					IsFile = true
				}

				docs = append(docs, Document{
					ID:      id,
					Content: content, // Adjust if needed based on how names are stored
					Description: func() string {
						if len(content) < 30 {
							return content
						}
						return content[:30] + "..."
					}(),
					IsFile: IsFile,
				})
			}
		}
	}

	return docs, ""
}

func listDocuments(c *gin.Context) {
	docs, err := azListDocs()
	if err != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
	}

	c.JSON(http.StatusOK, docs)
}

func updateDocument(c *gin.Context) {
	id := c.Param("id")
	client, err := getBlobServiceClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Azure client"})
		return
	}

	log.Println("Updating document", id)

	var form DocumentForm
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Message: "Invalid form data"})
		return
	}

	containerClient := client.ServiceClient().NewContainerClient("documents")
	blobClient := containerClient.NewBlockBlobClient(id)
	content := []byte(form.Content)

	// Update the document content
	_, err = blobClient.UploadStream(context.TODO(), io.NopCloser(bytes.NewReader(content)), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: fmt.Sprintf("Failed to update document: %v", err)})
		return
	}

	var fileID *string
	file, err := c.FormFile("file")
	if err == nil {
		fileName := fmt.Sprintf("%s_%s", id, file.Filename)
		fileClient := containerClient.NewBlockBlobClient(fileName)

		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Message: fmt.Sprintf("Failed to open file: %v", err)})
			return
		}
		defer f.Close()

		_, err = fileClient.UploadStream(context.TODO(), f, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Message: fmt.Sprintf("Failed to upload file: %v", err)})
			return
		}

		fileID = &fileName
	}

	c.JSON(http.StatusOK, Document{
		ID:      id,
		Content: form.Content,
		FileID:  fileID,
	})
}

func deleteDocument(c *gin.Context) {
	id := c.Param("id")
	log.Println("Deleting document", id)
	client, err := getBlobServiceClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Azure client"})
		return
	}

	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlockBlobClient(id)
	_, err = blobClient.Delete(context.TODO(), nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document deleted successfully"})
}

func mainIndex(c *gin.Context) {
	// var Documents []Document
	docs, err := azListDocs()
	if err != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title":     "Documents",
		"Documents": docs,
	})
}

func main() {
	if accountName == "" || accountKey == "" || containerName == "" {
		log.Fatal("Azure Storage credentials are not set")
	}

	r := gin.Default()

	// Add CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.LoadHTMLGlob("templates/*")
	r.GET("/", mainIndex)

	apiRoutes := r.Group("/api")
	{
		apiRoutes.GET("/documents", listDocuments)
		apiRoutes.POST("/documents", createDocument)
		apiRoutes.GET("/documents/download/:id", downloadDocument)
		apiRoutes.GET("/documents/:id", getDocument)
		apiRoutes.PUT("/documents/:id", updateDocument)
		apiRoutes.DELETE("/documents/:id", deleteDocument)
	}

	log.Println("Starting server on :8080")
	r.Run(":8080")
}
