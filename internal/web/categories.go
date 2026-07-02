package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zackb/minfin/internal/store"
)

type categoriesView struct {
	viewBase
	From       string // yyyy-mm-dd
	To         string
	SpendJSON  template.JS
	IncomeJSON template.JS
	Spend      []store.CategoryStat
	Income     []store.CategoryStat
	Categories []store.Category
	Rules      []store.Rule
}

// pieData is the chart-ready shape consumed by categories.html.
type pieData struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
	Colors []string  `json:"colors"`
}

func toPie(stats []store.CategoryStat) pieData {
	p := pieData{}
	for i, st := range stats {
		p.Labels = append(p.Labels, st.Category)
		p.Values = append(p.Values, st.Amount)
		c := st.Color
		if c == "" {
			c = palette[i%len(palette)]
		}
		p.Colors = append(p.Colors, c)
	}
	return p
}

// palette mirrors the spending chart so uncategorized/unseeded slices still get a color.
var palette = []string{"#26c6da", "#7e57c2", "#66bb6a", "#ffa726", "#ef5350", "#42a5f5", "#ec407a", "#26a69a"}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	v := categoriesView{viewBase: s.base(w, r, "categories")}
	if !v.Connected {
		s.render(w, "categories", v)
		return
	}
	pid := portfolioID(r)
	q := r.URL.Query()
	now := time.Now()
	v.To = orDefault(q.Get("to"), now.Format(dateLayout))
	v.From = orDefault(q.Get("from"), now.AddDate(0, 0, -30).Format(dateLayout))
	start := parseDate(v.From, now.AddDate(0, 0, -30))
	end := parseDate(v.To, now).AddDate(0, 0, 1)

	if spend, err := s.store.SpendByCategory(pid, start, end); err != nil {
		v.failed("categories: spend", err)
	} else {
		v.Spend = spend
		v.SpendJSON = marshalPie(spend)
	}
	if income, err := s.store.IncomeByCategory(pid, start, end); err != nil {
		v.failed("categories: income", err)
	} else {
		v.Income = income
		v.IncomeJSON = marshalPie(income)
	}
	if cats, err := s.store.Categories(pid); err != nil {
		v.failed("categories: list", err)
	} else {
		v.Categories = cats
	}
	if rules, err := s.store.Rules(pid); err != nil {
		v.failed("categories: rules", err)
	} else {
		v.Rules = rules
	}
	s.render(w, "categories", v)
}

func marshalPie(stats []store.CategoryStat) template.JS {
	b, _ := json.Marshal(toPie(stats))
	return template.JS(b)
}

// handleTxnCategory assigns a category to one transaction, optionally remembering
// the payee→category mapping as a rule.
func (s *Server) handleTxnCategory(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	category := r.FormValue("category")
	if id == "" {
		flash(w, "That transaction couldn't be categorized.")
		http.Redirect(w, r, safeReferer(r, "/transactions"), http.StatusSeeOther)
		return
	}
	pid := portfolioID(r)
	if err := s.store.SetTxnCategory(pid, id, category); err != nil {
		log.Printf("txn category: %v", err)
		flash(w, "That transaction couldn't be categorized.")
		http.Redirect(w, r, safeReferer(r, "/transactions"), http.StatusSeeOther)
		return
	}
	if r.FormValue("remember") != "" && category != "" {
		pattern := strings.TrimSpace(orDefault(r.FormValue("pattern"), r.FormValue("payee")))
		if pattern != "" {
			if err := s.store.AddRule(pid, pattern, category); err != nil {
				serverError(w, r.URL.Path, err)
				return
			}
		}
	}
	http.Redirect(w, r, safeReferer(r, "/transactions"), http.StatusSeeOther)
}

func (s *Server) handleCategoryAdd(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name != "" {
		if err := s.store.AddCategory(portfolioID(r), name); err != nil {
			serverError(w, r.URL.Path, err)
			return
		}
	}
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}

func (s *Server) handleCategoryDelete(w http.ResponseWriter, r *http.Request) {
	if name := r.FormValue("name"); name != "" {
		if err := s.store.DeleteCategory(portfolioID(r), name); err != nil {
			serverError(w, r.URL.Path, err)
			return
		}
	}
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}

func (s *Server) handleCategoryExclude(w http.ResponseWriter, r *http.Request) {
	if name := r.FormValue("name"); name != "" {
		if err := s.store.SetCategoryExclude(portfolioID(r), name, r.FormValue("exclude") == "1"); err != nil {
			serverError(w, r.URL.Path, err)
			return
		}
	}
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}

func (s *Server) handleRuleAdd(w http.ResponseWriter, r *http.Request) {
	pattern := strings.TrimSpace(r.FormValue("pattern"))
	category := r.FormValue("category")
	if pattern != "" && category != "" {
		if err := s.store.AddRule(portfolioID(r), pattern, category); err != nil {
			log.Printf("rule add: %v", err)
			flash(w, "That rule couldn't be saved.")
			http.Redirect(w, r, "/categories", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}

func (s *Server) handleRuleDelete(w http.ResponseWriter, r *http.Request) {
	if id, err := strconv.ParseInt(r.FormValue("id"), 10, 64); err == nil {
		if err := s.store.DeleteRule(portfolioID(r), id); err != nil {
			serverError(w, r.URL.Path, err)
			return
		}
	}
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}

func (s *Server) handleRecategorize(w http.ResponseWriter, r *http.Request) {
	// Manual button overwrites: re-apply rules over already-categorized rows so
	// stale/mis-matched categories get corrected.
	if _, err := s.store.ApplyRules(portfolioID(r), true); err != nil {
		serverError(w, r.URL.Path, err)
		return
	}
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}
