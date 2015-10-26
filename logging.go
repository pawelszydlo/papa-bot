package main

import (
	"log"
	"os"
)

var (
	linfo    *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	lwarn 	*log.Logger = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	lerror   *log.Logger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
)
