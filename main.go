package main

import (
	"log"
	"os"
)

func main() {
	dirs := &DirectoryNode{
		PathPart: "/home/egor_usual/fsyncd",
		Nested: map[string]*DirectoryNode{
			"a": &DirectoryNode{
				PathPart: "a",
				Nested: map[string]*DirectoryNode{
					"c": &DirectoryNode{
						PathPart: "c",
					},
					"d": &DirectoryNode{
						PathPart: "d",
						Nested: map[string]*DirectoryNode{
							"e": &DirectoryNode{
								PathPart: "e",
							},
						},
					},
				},
			},
			"b": &DirectoryNode{
				PathPart: "b",
			},
		},
	}

	if err := CreateDirectoriesBFS(dirs, os.ModeDir|0755); err != nil {
		log.Fatal(err)
	}
}
