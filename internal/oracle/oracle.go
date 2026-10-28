package oracle

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/sijms/go-ora/v2"
)

type ConnParams struct {
	Hosts   []string
	Service string
	Schema  string
	Pass    string
}

type Tramite struct {
	DIG_TRAMITE        int64
	DIG_CEDULA         string
	DIG_DEPENDIENTE_01 string
	DIG_DEPENDIENTE_02 string
	DIG_MENOR_EDAD     string
	DIG_FECHA_PLANILLA string
	DIG_FECHA_HASTA    string
	DIG_FECHA_ALTA     string
	Tandas             int
}

type PDFJob struct {
	Cedula string
	Fecha  string
	PDFNum int
	Tanda  int
}

func Connect(p ConnParams) (*sql.DB, error) {
	host := p.Hosts[0]
	connStr := buildOracleDSN(p.Schema, p.Pass, host, p.Service)
	db, err := sql.Open("oracle", connStr)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	return db, db.Ping()
}

func GetTramite(db *sql.DB, schema string, digTramite int64) (*Tramite, error) {
	q := fmt.Sprintf(`SELECT DIG_TRAMITE, DIG_CEDULA, DIG_DEPENDIENTE_01, DIG_DEPENDIENTE_02,
		DIG_MENOR_EDAD, DIG_FECHA_PLANILLA, DIG_FECHA_HASTA, DIG_FECHA_ALTA
		FROM %s.digitalizacion WHERE DIG_TRAMITE = :1`, schema)

	t := &Tramite{}
	var dep01, dep02 sql.NullString
	var planilla, hasta, alta sql.NullTime
	err := db.QueryRow(q, digTramite).Scan(
		&t.DIG_TRAMITE, &t.DIG_CEDULA, &dep01, &dep02,
		&t.DIG_MENOR_EDAD, &planilla, &hasta, &alta)
	if err != nil {
		return nil, fmt.Errorf("trámite %d no encontrado: %w", digTramite, err)
	}
	if dep01.Valid {
		t.DIG_DEPENDIENTE_01 = strings.TrimSpace(dep01.String)
	}
	if dep02.Valid {
		t.DIG_DEPENDIENTE_02 = strings.TrimSpace(dep02.String)
	}
	if planilla.Valid {
		t.DIG_FECHA_PLANILLA = planilla.Time.Format("02-01-2006")
	}
	if hasta.Valid {
		t.DIG_FECHA_HASTA = hasta.Time.Format("02-01-2006")
	}
	if alta.Valid {
		t.DIG_FECHA_ALTA = alta.Time.Format("02-01-2006")
	}
	t.Tandas = calcTandas(t)
	return t, nil
}

func calcTandas(t *Tramite) int {
	if t.DIG_FECHA_ALTA == "" {
		return 1
	}
	return 2
}

func (t *Tramite) BuildJobs() []PDFJob {
	var jobs []PDFJob
	pdfNum := 1

	cedulas := []string{t.DIG_CEDULA}
	if t.DIG_DEPENDIENTE_01 != "" {
		cedulas = append(cedulas, t.DIG_DEPENDIENTE_01)
	}
	if t.DIG_DEPENDIENTE_02 != "" {
		cedulas = append(cedulas, t.DIG_DEPENDIENTE_02)
	}

	if t.Tandas == 1 {
		for _, c := range cedulas {
			jobs = append(jobs, PDFJob{Cedula: c, Fecha: t.DIG_FECHA_HASTA, PDFNum: pdfNum, Tanda: 1})
			pdfNum++
		}
	} else {
		for _, c := range cedulas {
			jobs = append(jobs, PDFJob{Cedula: c, Fecha: t.DIG_FECHA_HASTA, PDFNum: pdfNum, Tanda: 1})
			pdfNum++
		}
		for _, c := range cedulas {
			jobs = append(jobs, PDFJob{Cedula: c, Fecha: t.DIG_FECHA_ALTA, PDFNum: pdfNum, Tanda: 2})
			pdfNum++
		}
	}
	return jobs
}

// GetGeneraciones retorna los DIG_ID_GENERACION distintos para un período
func GetGeneraciones(db *sql.DB, schema string, periodo string) ([]int64, error) {
	q := fmt.Sprintf(`SELECT DISTINCT DIG_ID_GENERACION FROM %s.digitalizacion 
		WHERE FE_PLA_ANIOMES = :1 AND DIG_PLANILLADO = 'S' AND DIG_ID_GENERACION IS NOT NULL 
		ORDER BY DIG_ID_GENERACION`, schema)
	rows, err := db.Query(q, periodo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []int64
	for rows.Next() {
		var g int64
		if err := rows.Scan(&g); err != nil {
			continue
		}
		result = append(result, g)
	}
	return result, nil
}

// GetPeriodos retorna los FE_PLA_ANIOMES disponibles (últimos primero)
func GetPeriodos(db *sql.DB, schema string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf(`SELECT * FROM (
		SELECT FE_PLA_ANIOMES FROM %s.digitalizacion 
		WHERE FE_PLA_ANIOMES IS NOT NULL 
		GROUP BY FE_PLA_ANIOMES ORDER BY FE_PLA_ANIOMES DESC
	) WHERE ROWNUM <= 24`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

// GetPlanillasByGeneracion retorna los trámites para un período+generación con DIG_PLANILLADO='S'
func GetPlanillasByGeneracion(db *sql.DB, schema string, periodo string, generacion int64) ([]Tramite, error) {
	q := fmt.Sprintf(`SELECT DIG_TRAMITE, DIG_CEDULA, DIG_DEPENDIENTE_01, DIG_DEPENDIENTE_02,
		DIG_MENOR_EDAD, DIG_FECHA_PLANILLA, DIG_FECHA_HASTA, DIG_FECHA_ALTA
		FROM %s.digitalizacion 
		WHERE FE_PLA_ANIOMES = :1 AND DIG_ID_GENERACION = :2 AND DIG_PLANILLADO = 'S'
		ORDER BY DIG_TRAMITE`, schema)

	rows, err := db.Query(q, periodo, generacion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Tramite
	for rows.Next() {
		t := Tramite{}
		var dep01, dep02 sql.NullString
		var planilla, hasta, alta sql.NullTime
		if err := rows.Scan(&t.DIG_TRAMITE, &t.DIG_CEDULA, &dep01, &dep02,
			&t.DIG_MENOR_EDAD, &planilla, &hasta, &alta); err != nil {
			continue
		}
		if dep01.Valid {
			t.DIG_DEPENDIENTE_01 = strings.TrimSpace(dep01.String)
		}
		if dep02.Valid {
			t.DIG_DEPENDIENTE_02 = strings.TrimSpace(dep02.String)
		}
		if planilla.Valid {
			t.DIG_FECHA_PLANILLA = planilla.Time.Format("02-01-2006")
		}
		if hasta.Valid {
			t.DIG_FECHA_HASTA = hasta.Time.Format("02-01-2006")
		}
		if alta.Valid {
			t.DIG_FECHA_ALTA = alta.Time.Format("02-01-2006")
		}
		t.Tandas = calcTandas(&t)
		result = append(result, t)
	}
	return result, nil
}

// GetIdTramites retorna los DIG_ID_TRAMITE distintos para un período
func GetIdTramites(db *sql.DB, schema string, periodo string) ([]int64, error) {
	q := fmt.Sprintf(`SELECT DISTINCT DIG_ID_TRAMITE FROM %s.digitalizacion 
		WHERE FE_PLA_ANIOMES = :1 AND DIG_PLANILLADO = 'S' AND DIG_ID_TRAMITE IS NOT NULL 
		ORDER BY DIG_ID_TRAMITE`, schema)
	rows, err := db.Query(q, periodo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []int64
	for rows.Next() {
		var g int64
		if err := rows.Scan(&g); err != nil {
			continue
		}
		result = append(result, g)
	}
	return result, nil
}

// GetPlanillasByIdTramite retorna los trámites para un período+id_tramite con DIG_PLANILLADO='S'
func GetPlanillasByIdTramite(db *sql.DB, schema string, periodo string, idTramite int64) ([]Tramite, error) {
	q := fmt.Sprintf(`SELECT DIG_TRAMITE, DIG_CEDULA, DIG_DEPENDIENTE_01, DIG_DEPENDIENTE_02,
		DIG_MENOR_EDAD, DIG_FECHA_PLANILLA, DIG_FECHA_HASTA, DIG_FECHA_ALTA
		FROM %s.digitalizacion 
		WHERE FE_PLA_ANIOMES = :1 AND DIG_ID_TRAMITE = :2 AND DIG_PLANILLADO = 'S'
		ORDER BY DIG_TRAMITE`, schema)

	rows, err := db.Query(q, periodo, idTramite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Tramite
	for rows.Next() {
		t := Tramite{}
		var dep01, dep02 sql.NullString
		var planilla, hasta, alta sql.NullTime
		if err := rows.Scan(&t.DIG_TRAMITE, &t.DIG_CEDULA, &dep01, &dep02,
			&t.DIG_MENOR_EDAD, &planilla, &hasta, &alta); err != nil {
			continue
		}
		if dep01.Valid {
			t.DIG_DEPENDIENTE_01 = strings.TrimSpace(dep01.String)
		}
		if dep02.Valid {
			t.DIG_DEPENDIENTE_02 = strings.TrimSpace(dep02.String)
		}
		if planilla.Valid {
			t.DIG_FECHA_PLANILLA = planilla.Time.Format("02-01-2006")
		}
		if hasta.Valid {
			t.DIG_FECHA_HASTA = hasta.Time.Format("02-01-2006")
		}
		if alta.Valid {
			t.DIG_FECHA_ALTA = alta.Time.Format("02-01-2006")
		}
		t.Tandas = calcTandas(&t)
		result = append(result, t)
	}
	return result, nil
}
