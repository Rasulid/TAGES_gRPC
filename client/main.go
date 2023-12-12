package main

import (
	"context"
	"google.golang.org/grpc"
	"io"
	"io/ioutil"
	"log"
	pb "tages.rasulabduvaitov.net/tages.rasulabduvaitov.net"
	"time"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Не удалось подключиться: %v", err)
	}
	defer conn.Close()
	c := pb.NewFileServiceClient(conn)

	testUploadFile(c)

	testGetFileList(c)

	testDownloadFile(c)
}

func testUploadFile(c pb.FileServiceClient) {
	data, err := ioutil.ReadFile("client/test.jpg")
	if err != nil {
		log.Fatalf("Ошибка при чтении файла: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.UploadFile(ctx, &pb.FileRequest{FileContent: data, FileName: "test.jpg"})
	if err != nil {
		log.Fatalf("Не удалось загрузить файл: %v", err)
	}
	log.Printf("Ответ сервера на запрос загрузки файла: %s", r.GetMessage())
}

func testGetFileList(c pb.FileServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.GetFileList(ctx, &pb.Empty{})
	if err != nil {
		log.Fatalf("Не удалось получить список файлов: %v", err)
	}

	log.Println("Список файлов:")
	for _, file := range r.GetFiles() {
		log.Printf("Имя файла: %s | Дата создания: %s | Дата обновления: %s",
			file.GetFileName(), file.GetCreatedAt(), file.GetUpdatedAt())
	}
}

func testDownloadFile(c pb.FileServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.DownloadFile(ctx, &pb.FileRequest{FileName: "test.jpg"})
	if err != nil {
		log.Fatalf("Не удалось скачать файл: %v", err)
	}
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Ошибка при скачивании: %v", err)
		}
	}
	log.Println("Скачивание файла test.jpg завершено")
}
