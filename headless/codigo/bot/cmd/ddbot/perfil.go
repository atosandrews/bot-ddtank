package main

// Diagnóstico de CPU (09/11/2026): com DDBOT_CPU_PROFILE=<arquivo>, grava um perfil de CPU de
// 120 s, começando 180 s depois do início (pula a carga inicial). Sem a variável, não faz nada.

import (
	"os"
	"runtime/pprof"
	"time"
)

func init() {
	caminho := os.Getenv("DDBOT_CPU_PROFILE")
	if caminho == "" {
		return
	}

	go func() {
		time.Sleep(180 * time.Second)
		arquivo, err := os.Create(caminho + ".parcial")
		if err != nil {
			return
		}
		if err := pprof.StartCPUProfile(arquivo); err != nil {
			arquivo.Close()
			return
		}
		time.Sleep(120 * time.Second)
		pprof.StopCPUProfile()
		arquivo.Close()
		os.Rename(caminho+".parcial", caminho)
	}()
}
