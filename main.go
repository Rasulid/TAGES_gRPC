package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "tages.rasulabduvaitov.net/tages.rasulabduvaitov.net"
)

const (
	maxConcurrentUploads   = 10
	maxConcurrentDownloads = 10
	maxConcurrentFileList  = 100
	fileDirectory          = "./files"
)

type server struct {
	pb.UnimplementedFileServiceServer
	uploadedFiles map[string]*fileInfo
	uploadLock    chan struct{}
	downloadLock  chan struct{}
	listLock      chan struct{}
	mu            sync.Mutex
}

type fileInfo struct {
	name      string
	createdAt time.Time
	updatedAt time.Time
}

func (s *server) UploadFile(ctx context.Context, req *pb.FileRequest) (*pb.FileResponse, error) {
	select {
	case <-s.uploadLock:
		defer func() { s.uploadLock <- struct{}{} }()
	default:
		return nil, status.Errorf(codes.ResourceExhausted, "maximum concurrent uploads reached")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := fmt.Sprintf("%s/%s", fileDirectory, req.FileName)
	newFile, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	defer newFile.Close()

	_, err = newFile.Write(req.FileContent)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	s.uploadedFiles[req.FileName] = &fileInfo{
		name:      req.FileName,
		createdAt: now,
		updatedAt: now,
	}

	return &pb.FileResponse{Message: "File uploaded successfully"}, nil
}

func (s *server) GetFileList(ctx context.Context, empty *pb.Empty) (*pb.FileList, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	var files []*pb.File
	for _, fileInfo := range s.uploadedFiles {
		files = append(files, &pb.File{
			FileName:  fileInfo.name,
			CreatedAt: fileInfo.createdAt.Format(time.RFC3339),
			UpdatedAt: fileInfo.updatedAt.Format(time.RFC3339),
		})
	}

	return &pb.FileList{Files: files}, nil
}

func (s *server) DownloadFile(req *pb.FileRequest, stream pb.FileService_DownloadFileServer) error {
	select {
	case <-s.downloadLock:
		defer func() { s.downloadLock <- struct{}{} }()
	default:
		return status.Errorf(codes.ResourceExhausted, "maximum concurrent downloads reached")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.uploadedFiles[req.FileName]
	if !ok {
		return status.Errorf(codes.NotFound, "file not found")
	}

	file, err := os.Open(fmt.Sprintf("%s/%s", fileDirectory, req.FileName))
	if err != nil {
		return err
	}
	defer file.Close()

	chunkSize := 1024
	buffer := make([]byte, chunkSize)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		err = stream.Send(&pb.FileChunk{Content: buffer[:n]})
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := os.MkdirAll(fileDirectory, os.ModePerm); err != nil {
		log.Fatalf("failed to create file directory: %v", err)
	}

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	serverInstance := &server{
		uploadedFiles: make(map[string]*fileInfo),
		uploadLock:    make(chan struct{}, maxConcurrentUploads),
		downloadLock:  make(chan struct{}, maxConcurrentDownloads),
		listLock:      make(chan struct{}, maxConcurrentFileList),
	}

	for i := 0; i < maxConcurrentUploads; i++ {
		serverInstance.uploadLock <- struct{}{}
	}
	for i := 0; i < maxConcurrentDownloads; i++ {
		serverInstance.downloadLock <- struct{}{}
	}
	for i := 0; i < maxConcurrentFileList; i++ {
		serverInstance.listLock <- struct{}{}
	}

	pb.RegisterFileServiceServer(srv, serverInstance)
	reflection.Register(srv)

	log.Println("Server started on port 50051...")
	if err := srv.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
