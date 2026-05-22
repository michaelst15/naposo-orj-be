package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db           *pgxpool.Pool
	allowAll     bool
	allowOrigins map[string]struct{}
}

type Member struct {
	ID       string `json:"id"`
	Nama     string `json:"nama"`
	Angkatan string `json:"angkatan"`
	Posisi   string `json:"posisi"`
	Foto     string `json:"foto"`
	Bio      string `json:"bio"`
	Kontak   string `json:"kontak"`
}

type MemberCreate struct {
	Nama     string `json:"nama"`
	Angkatan string `json:"angkatan"`
	Posisi   string `json:"posisi"`
	Foto     string `json:"foto"`
	Bio      string `json:"bio"`
	Kontak   string `json:"kontak"`
}

type Pengurus struct {
	ID     string `json:"id"`
	Nama   string `json:"nama"`
	Posisi string `json:"posisi"`
	Foto   string `json:"foto"`
	Bio    string `json:"bio"`
	Urutan int    `json:"urutan"`
}

type PengurusCreate struct {
	Nama   string `json:"nama"`
	Posisi string `json:"posisi"`
	Foto   string `json:"foto"`
	Bio    string `json:"bio"`
	Urutan int    `json:"urutan"`
}

type Activity struct {
	ID        string `json:"id"`
	Judul     string `json:"judul"`
	Deskripsi string `json:"deskripsi"`
	Tanggal   string `json:"tanggal"`
	Status    string `json:"status"`
	Gambar    string `json:"gambar"`
}

type ActivityCreate struct {
	Judul     string `json:"judul"`
	Deskripsi string `json:"deskripsi"`
	Tanggal   string `json:"tanggal"`
	Status    string `json:"status"`
	Gambar    string `json:"gambar"`
}

type Gallery struct {
	ID        string `json:"id"`
	Judul     string `json:"judul"`
	Gambar    string `json:"gambar"`
	Tanggal   string `json:"tanggal"`
	Deskripsi string `json:"deskripsi"`
}

type GalleryCreate struct {
	Judul     string `json:"judul"`
	Gambar    string `json:"gambar"`
	Tanggal   string `json:"tanggal"`
	Deskripsi string `json:"deskripsi"`
}

type Registration struct {
	ID            string `json:"id"`
	Nama          string `json:"nama"`
	Email         string `json:"email"`
	Telepon       string `json:"telepon"`
	Angkatan      string `json:"angkatan"`
	Alasan        string `json:"alasan"`
	TanggalDaftar string `json:"tanggal_daftar"`
}

type RegistrationCreate struct {
	Nama     string `json:"nama"`
	Email    string `json:"email"`
	Telepon  string `json:"telepon"`
	Angkatan string `json:"angkatan"`
	Alasan   string `json:"alasan"`
}

func main() {
	ctx := context.Background()

	app := &App{
		allowOrigins: map[string]struct{}{},
	}
	app.configureCORS()

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	dbURL = strings.Trim(dbURL, "\"'")
	if dbURL != "" {
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Fatalf("db connect failed: %v", err)
		}
		app.db = pool
		if err := ensureSchema(ctx, pool); err != nil {
			log.Fatalf("ensure schema failed: %v", err)
		}
	}

	r := chi.NewRouter()
	r.Use(app.corsMiddleware)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	r.Route("/api", func(api chi.Router) {
		api.Get("/", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"message": "Naposo ORJ API"})
		})

		api.Get("/members", app.getMembers)
		api.Post("/members", app.createMember)
		api.Delete("/members/{memberID}", app.deleteMember)

		api.Get("/pengurus", app.getPengurus)
		api.Post("/pengurus", app.createPengurus)

		api.Get("/activities", app.getActivities)
		api.Post("/activities", app.createActivity)

		api.Get("/gallery", app.getGallery)
		api.Post("/gallery", app.createGallery)

		api.Post("/registrations", app.createRegistration)
		api.Get("/registrations", app.getRegistrations)

		api.Post("/seed", app.seedData)
	})

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func (a *App) configureCORS() {
	raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	if raw == "" {
		a.allowAll = true
		return
	}
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		o := strings.TrimSpace(p)
		if o == "" {
			continue
		}
		if o == "*" {
			a.allowAll = true
			a.allowOrigins = map[string]struct{}{}
			return
		}
		a.allowOrigins[o] = struct{}{}
	}
	if len(a.allowOrigins) == 0 {
		a.allowAll = true
	}
}

func (a *App) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if a.allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			if _, ok := a.allowOrigins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

func (a *App) requireDB(w http.ResponseWriter) bool {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"detail": "DATABASE_URL belum di-set."})
		return false
	}
	return true
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS members (
		  id TEXT PRIMARY KEY,
		  nama TEXT NOT NULL,
		  angkatan TEXT NOT NULL,
		  posisi TEXT NOT NULL,
		  foto TEXT NOT NULL,
		  bio TEXT NOT NULL,
		  kontak TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pengurus (
		  id TEXT PRIMARY KEY,
		  nama TEXT NOT NULL,
		  posisi TEXT NOT NULL,
		  foto TEXT NOT NULL,
		  bio TEXT NOT NULL,
		  urutan INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS activities (
		  id TEXT PRIMARY KEY,
		  judul TEXT NOT NULL,
		  deskripsi TEXT NOT NULL,
		  tanggal TEXT NOT NULL,
		  status TEXT NOT NULL,
		  gambar TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS gallery (
		  id TEXT PRIMARY KEY,
		  judul TEXT NOT NULL,
		  gambar TEXT NOT NULL,
		  tanggal TEXT NOT NULL,
		  deskripsi TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS registrations (
		  id TEXT PRIMARY KEY,
		  nama TEXT NOT NULL,
		  email TEXT NOT NULL,
		  telepon TEXT NOT NULL,
		  angkatan TEXT NOT NULL,
		  alasan TEXT NOT NULL,
		  tanggal_daftar TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) getMembers(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	angkatan := strings.TrimSpace(r.URL.Query().Get("angkatan"))
	posisi := strings.TrimSpace(r.URL.Query().Get("posisi"))
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	where := []string{}
	args := []any{}

	if angkatan != "" {
		args = append(args, angkatan)
		where = append(where, fmt.Sprintf("angkatan = $%d", len(args)))
	}
	if posisi != "" {
		args = append(args, posisi)
		where = append(where, fmt.Sprintf("posisi = $%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, fmt.Sprintf("nama ILIKE $%d", len(args)))
	}

	sql := "SELECT id, nama, angkatan, posisi, foto, bio, kontak FROM members"
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}

	rows, err := a.db.Query(ctx, sql, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	defer rows.Close()

	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Nama, &m.Angkatan, &m.Posisi, &m.Foto, &m.Bio, &m.Kontak); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) createMember(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	var in MemberCreate
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	m := Member{
		ID:       uuid.NewString(),
		Nama:     strings.TrimSpace(in.Nama),
		Angkatan: strings.TrimSpace(in.Angkatan),
		Posisi:   strings.TrimSpace(in.Posisi),
		Foto:     strings.TrimSpace(in.Foto),
		Bio:      strings.TrimSpace(in.Bio),
		Kontak:   strings.TrimSpace(in.Kontak),
	}

	if m.Nama == "" || m.Angkatan == "" || m.Posisi == "" || m.Foto == "" || m.Bio == "" || m.Kontak == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "field wajib belum lengkap"})
		return
	}

	_, err := a.db.Exec(ctx, `
		INSERT INTO members (id, nama, angkatan, posisi, foto, bio, kontak)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, m.ID, m.Nama, m.Angkatan, m.Posisi, m.Foto, m.Bio, m.Kontak)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, m)
}

func (a *App) deleteMember(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()
	id := chi.URLParam(r, "memberID")
	id = strings.TrimSpace(id)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "member_id kosong"})
		return
	}

	ct, err := a.db.Exec(ctx, "DELETE FROM members WHERE id = $1", id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	if ct.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Member deleted"})
}

func (a *App) getPengurus(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	rows, err := a.db.Query(ctx, "SELECT id, nama, posisi, foto, bio, urutan FROM pengurus ORDER BY urutan ASC")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	defer rows.Close()

	out := []Pengurus{}
	for rows.Next() {
		var p Pengurus
		if err := rows.Scan(&p.ID, &p.Nama, &p.Posisi, &p.Foto, &p.Bio, &p.Urutan); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) createPengurus(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	var in PengurusCreate
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	p := Pengurus{
		ID:     uuid.NewString(),
		Nama:   strings.TrimSpace(in.Nama),
		Posisi: strings.TrimSpace(in.Posisi),
		Foto:   strings.TrimSpace(in.Foto),
		Bio:    strings.TrimSpace(in.Bio),
		Urutan: in.Urutan,
	}

	if p.Nama == "" || p.Posisi == "" || p.Foto == "" || p.Bio == "" || p.Urutan == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "field wajib belum lengkap"})
		return
	}

	_, err := a.db.Exec(ctx, `
		INSERT INTO pengurus (id, nama, posisi, foto, bio, urutan)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, p.ID, p.Nama, p.Posisi, p.Foto, p.Bio, p.Urutan)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, p)
}

func (a *App) getActivities(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	sql := "SELECT id, judul, deskripsi, tanggal, status, gambar FROM activities"
	args := []any{}
	if status != "" {
		args = append(args, status)
		sql += " WHERE status = $1"
	}

	rows, err := a.db.Query(ctx, sql, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	defer rows.Close()

	out := []Activity{}
	for rows.Next() {
		var aItem Activity
		if err := rows.Scan(&aItem.ID, &aItem.Judul, &aItem.Deskripsi, &aItem.Tanggal, &aItem.Status, &aItem.Gambar); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
		out = append(out, aItem)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) createActivity(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	var in ActivityCreate
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	act := Activity{
		ID:        uuid.NewString(),
		Judul:     strings.TrimSpace(in.Judul),
		Deskripsi: strings.TrimSpace(in.Deskripsi),
		Tanggal:   strings.TrimSpace(in.Tanggal),
		Status:    strings.TrimSpace(in.Status),
		Gambar:    strings.TrimSpace(in.Gambar),
	}

	if act.Judul == "" || act.Deskripsi == "" || act.Tanggal == "" || act.Status == "" || act.Gambar == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "field wajib belum lengkap"})
		return
	}

	_, err := a.db.Exec(ctx, `
		INSERT INTO activities (id, judul, deskripsi, tanggal, status, gambar)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, act.ID, act.Judul, act.Deskripsi, act.Tanggal, act.Status, act.Gambar)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, act)
}

func (a *App) getGallery(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	rows, err := a.db.Query(ctx, "SELECT id, judul, gambar, tanggal, deskripsi FROM gallery")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	defer rows.Close()

	out := []Gallery{}
	for rows.Next() {
		var g Gallery
		if err := rows.Scan(&g.ID, &g.Judul, &g.Gambar, &g.Tanggal, &g.Deskripsi); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
		out = append(out, g)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) createGallery(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	var in GalleryCreate
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	g := Gallery{
		ID:        uuid.NewString(),
		Judul:     strings.TrimSpace(in.Judul),
		Gambar:    strings.TrimSpace(in.Gambar),
		Tanggal:   strings.TrimSpace(in.Tanggal),
		Deskripsi: strings.TrimSpace(in.Deskripsi),
	}

	if g.Judul == "" || g.Gambar == "" || g.Tanggal == "" || g.Deskripsi == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "field wajib belum lengkap"})
		return
	}

	_, err := a.db.Exec(ctx, `
		INSERT INTO gallery (id, judul, gambar, tanggal, deskripsi)
		VALUES ($1, $2, $3, $4, $5)
	`, g.ID, g.Judul, g.Gambar, g.Tanggal, g.Deskripsi)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, g)
}

func (a *App) createRegistration(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	var in RegistrationCreate
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	reg := Registration{
		ID:       uuid.NewString(),
		Nama:     strings.TrimSpace(in.Nama),
		Email:    strings.TrimSpace(in.Email),
		Telepon:  strings.TrimSpace(in.Telepon),
		Angkatan: strings.TrimSpace(in.Angkatan),
		Alasan:   strings.TrimSpace(in.Alasan),
	}

	if reg.Nama == "" || reg.Email == "" || reg.Telepon == "" || reg.Angkatan == "" || reg.Alasan == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "field wajib belum lengkap"})
		return
	}

	var ts time.Time
	err := a.db.QueryRow(ctx, `
		INSERT INTO registrations (id, nama, email, telepon, angkatan, alasan)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING tanggal_daftar
	`, reg.ID, reg.Nama, reg.Email, reg.Telepon, reg.Angkatan, reg.Alasan).Scan(&ts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	reg.TanggalDaftar = ts.UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusOK, reg)
}

func (a *App) getRegistrations(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	rows, err := a.db.Query(ctx, `
		SELECT id, nama, email, telepon, angkatan, alasan, tanggal_daftar
		FROM registrations
		ORDER BY tanggal_daftar DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	defer rows.Close()

	out := []Registration{}
	for rows.Next() {
		var reg Registration
		var ts time.Time
		if err := rows.Scan(&reg.ID, &reg.Nama, &reg.Email, &reg.Telepon, &reg.Angkatan, &reg.Alasan, &ts); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
		reg.TanggalDaftar = ts.UTC().Format(time.RFC3339)
		out = append(out, reg)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) seedData(w http.ResponseWriter, r *http.Request) {
	if !a.requireDB(w) {
		return
	}
	ctx := r.Context()

	tx, err := a.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureSchemaTx(ctx, tx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	_, err = tx.Exec(ctx, "TRUNCATE TABLE members, pengurus, activities, gallery, registrations")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	pengurusRows := []Pengurus{
		{ID: uuid.NewString(), Nama: "Andi Pratama", Posisi: "Ketua", Foto: "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=400", Bio: "Memimpin dengan visi membawa perubahan positif", Urutan: 1},
		{ID: uuid.NewString(), Nama: "Siti Nurhaliza", Posisi: "Wakil Ketua", Foto: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=400", Bio: "Berdedikasi untuk kemajuan organisasi", Urutan: 2},
		{ID: uuid.NewString(), Nama: "Budi Santoso", Posisi: "Sekretaris", Foto: "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=400", Bio: "Mengatur administrasi dengan rapi dan terstruktur", Urutan: 3},
		{ID: uuid.NewString(), Nama: "Dewi Lestari", Posisi: "Bendahara", Foto: "https://images.unsplash.com/photo-1438761681033-6461ffad8d80?w=400", Bio: "Mengelola keuangan dengan transparan", Urutan: 4},
		{ID: uuid.NewString(), Nama: "Rizky Firmansyah", Posisi: "Koordinator Acara", Foto: "https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?w=400", Bio: "Menciptakan event seru dan berkesan", Urutan: 5},
		{ID: uuid.NewString(), Nama: "Maya Indah", Posisi: "Koordinator Media", Foto: "https://images.unsplash.com/photo-1517841905240-472988babdf9?w=400", Bio: "Mengelola konten digital dan publikasi", Urutan: 6},
	}
	for _, p := range pengurusRows {
		_, err := tx.Exec(ctx, `
			INSERT INTO pengurus (id, nama, posisi, foto, bio, urutan)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, p.ID, p.Nama, p.Posisi, p.Foto, p.Bio, p.Urutan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
	}

	membersRows := []Member{
		{ID: uuid.NewString(), Nama: "Ahmad Fadhil", Angkatan: "2020", Posisi: "Anggota Aktif", Foto: "https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=400", Bio: "Senang berbagi dan belajar hal baru", Kontak: "ahmad.fadhil@email.com"},
		{ID: uuid.NewString(), Nama: "Linda Wijaya", Angkatan: "2021", Posisi: "Anggota Aktif", Foto: "https://images.unsplash.com/photo-1544005313-94ddf0286df2?w=400", Bio: "Passionate tentang social impact", Kontak: "linda.wijaya@email.com"},
		{ID: uuid.NewString(), Nama: "Dimas Ardianto", Angkatan: "2021", Posisi: "Volunteer", Foto: "https://images.unsplash.com/photo-1519085360753-af0119f7cbe7?w=400", Bio: "Suka membantu dan berkolaborasi", Kontak: "dimas.ardianto@email.com"},
		{ID: uuid.NewString(), Nama: "Putri Ayu", Angkatan: "2022", Posisi: "Anggota Baru", Foto: "https://images.unsplash.com/photo-1487412720507-e7ab37603c6f?w=400", Bio: "Excited untuk berkontribusi!", Kontak: "putri.ayu@email.com"},
		{ID: uuid.NewString(), Nama: "Arief Rahman", Angkatan: "2020", Posisi: "Volunteer", Foto: "https://images.unsplash.com/photo-1492562080023-ab3db95bfbce?w=400", Bio: "Believe in community power", Kontak: "arief.rahman@email.com"},
		{ID: uuid.NewString(), Nama: "Fatimah Zahra", Angkatan: "2022", Posisi: "Anggota Aktif", Foto: "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=400", Bio: "Love connecting with people", Kontak: "fatimah.zahra@email.com"},
		{ID: uuid.NewString(), Nama: "Hendra Gunawan", Angkatan: "2021", Posisi: "Anggota Aktif", Foto: "https://images.unsplash.com/photo-1463453091185-61582044d556?w=400", Bio: "Passionate about youth empowerment", Kontak: "hendra.gunawan@email.com"},
		{ID: uuid.NewString(), Nama: "Raisa Andriana", Angkatan: "2022", Posisi: "Anggota Baru", Foto: "https://images.unsplash.com/photo-1489424731084-a5d8b219a5bb?w=400", Bio: "Ready to make an impact", Kontak: "raisa.andriana@email.com"},
	}
	for _, m := range membersRows {
		_, err := tx.Exec(ctx, `
			INSERT INTO members (id, nama, angkatan, posisi, foto, bio, kontak)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, m.ID, m.Nama, m.Angkatan, m.Posisi, m.Foto, m.Bio, m.Kontak)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
	}

	activitiesRows := []Activity{
		{ID: uuid.NewString(), Judul: "Gathering Bulanan", Deskripsi: "Pertemuan rutin bulanan untuk sharing dan networking", Tanggal: "2026-02-15", Status: "upcoming", Gambar: "https://images.unsplash.com/photo-1511988617509-a57c8a288659?w=800"},
		{ID: uuid.NewString(), Judul: "Workshop Leadership", Deskripsi: "Pelatihan kepemimpinan untuk pengembangan diri", Tanggal: "2026-02-28", Status: "upcoming", Gambar: "https://images.unsplash.com/photo-1573497701240-345a300b8d36?w=800"},
		{ID: uuid.NewString(), Judul: "Bakti Sosial", Deskripsi: "Kegiatan sosial membantu masyarakat sekitar", Tanggal: "2026-01-20", Status: "past", Gambar: "https://images.unsplash.com/photo-1559027615-cd4628902d4a?w=800"},
		{ID: uuid.NewString(), Judul: "Fun Games Day", Deskripsi: "Hari penuh permainan seru dan team building", Tanggal: "2026-01-10", Status: "past", Gambar: "https://images.unsplash.com/photo-1702599057905-d3859caa8b61?w=800"},
		{ID: uuid.NewString(), Judul: "Seminar Karir", Deskripsi: "Tips dan trik membangun karir di era digital", Tanggal: "2026-03-10", Status: "upcoming", Gambar: "https://images.unsplash.com/photo-1540575467063-178a50c2df87?w=800"},
	}
	for _, aItem := range activitiesRows {
		_, err := tx.Exec(ctx, `
			INSERT INTO activities (id, judul, deskripsi, tanggal, status, gambar)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, aItem.ID, aItem.Judul, aItem.Deskripsi, aItem.Tanggal, aItem.Status, aItem.Gambar)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
	}

	galleryRows := []Gallery{
		{ID: uuid.NewString(), Judul: "Gathering Perdana 2025", Gambar: "https://images.unsplash.com/photo-1511988617509-a57c8a288659?w=800", Tanggal: "2025-12-15", Deskripsi: "Pertemuan pertama tahun 2025"},
		{ID: uuid.NewString(), Judul: "Workshop Kreatif", Gambar: "https://images.unsplash.com/photo-1573497701240-345a300b8d36?w=800", Tanggal: "2025-11-20", Deskripsi: "Pelatihan kreativitas bersama"},
		{ID: uuid.NewString(), Judul: "Outing Bersama", Gambar: "https://images.unsplash.com/photo-1702599057905-d3859caa8b61?w=800", Tanggal: "2025-10-10", Deskripsi: "Refreshing di alam terbuka"},
		{ID: uuid.NewString(), Judul: "Diskusi Komunitas", Gambar: "https://images.unsplash.com/photo-1590650046871-92c887180603?w=800", Tanggal: "2025-12-05", Deskripsi: "Sharing session tentang isu sosial"},
	}
	for _, g := range galleryRows {
		_, err := tx.Exec(ctx, `
			INSERT INTO gallery (id, judul, gambar, tanggal, deskripsi)
			VALUES ($1, $2, $3, $4, $5)
		`, g.ID, g.Judul, g.Gambar, g.Tanggal, g.Deskripsi)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "Data seeded successfully"})
}

func ensureSchemaTx(ctx context.Context, tx pgx.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS members (
		  id TEXT PRIMARY KEY,
		  nama TEXT NOT NULL,
		  angkatan TEXT NOT NULL,
		  posisi TEXT NOT NULL,
		  foto TEXT NOT NULL,
		  bio TEXT NOT NULL,
		  kontak TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pengurus (
		  id TEXT PRIMARY KEY,
		  nama TEXT NOT NULL,
		  posisi TEXT NOT NULL,
		  foto TEXT NOT NULL,
		  bio TEXT NOT NULL,
		  urutan INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS activities (
		  id TEXT PRIMARY KEY,
		  judul TEXT NOT NULL,
		  deskripsi TEXT NOT NULL,
		  tanggal TEXT NOT NULL,
		  status TEXT NOT NULL,
		  gambar TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS gallery (
		  id TEXT PRIMARY KEY,
		  judul TEXT NOT NULL,
		  gambar TEXT NOT NULL,
		  tanggal TEXT NOT NULL,
		  deskripsi TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS registrations (
		  id TEXT PRIMARY KEY,
		  nama TEXT NOT NULL,
		  email TEXT NOT NULL,
		  telepon TEXT NOT NULL,
		  angkatan TEXT NOT NULL,
		  alasan TEXT NOT NULL,
		  tanggal_daftar TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

var errNotFound = errors.New("not found")
