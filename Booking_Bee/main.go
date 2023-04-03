package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var tpl *template.Template
var tpl2 *template.Template
var db *sql.DB

type MoviesStruct struct {
	MovieID      int
	MovieName    string
	MovieDesc    string
	MoviePrice   float64
	CinemaHallID int
	PremierDate  string
	ImageUrl     string
}

type CinemaStruct struct {
	CinemaID       int
	CinemaName     string
	CinemaLocation string
	CinemaCapacity int
}

type TicketsStruct struct {
	TicketID     int
	SeatNo       int
	Availability int
	HallID       int
	MovieID      int
	CustomerID   int
}

type HistoryStruct struct {
	HistoryID  int
	CustomerID int
	TicketID   int
	CinemaID   int
	MovieID    int
	TimeStamp  string
}

type DashboardData struct {
	Movies     []MoviesStruct
	MoviesJSON string
	CinemaJSON string
	TicketJSON string
}

type Seat struct {
	UserID     string `json:"userId"`
	CinemaID   string `json:"cinemaId"`
	MovieID    string `json:"movieId"`
	SeatNumber string `json:"seatNumber"`
}

func main() {
	tpl, _ = template.ParseGlob("views/*.html")
	tpl2, _ = template.ParseGlob("views/screens/*.html")
	fs := http.FileServer(http.Dir("assets"))
	var err error
	db, err = sql.Open("mysql", "owen:ichimaruGin@tcp(localhost:3306)/Booking_Bee")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	http.Handle("/assets/", http.StripPrefix("/assets", fs))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/signup", signupHandler)
	http.HandleFunc("/login", loginHandler)

	//Middleware function to protect access to pages
	http.HandleFunc("/dashboard", requireLogin(dashHandler))
	http.HandleFunc("/payment", requireLogin(payHandler))
	http.HandleFunc("/pay", requireLogin(monHandler))
	http.HandleFunc("/history", requireLogin(historyHandler))
	http.HandleFunc("/deleteHistory", deleteHistoryHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.ListenAndServe(":8080", nil)
}

// Middleware function to require login for protected pages
func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if session cookie exists
		_, err := r.Cookie("user")
		if err != nil {
			// Session cookie does not exist, redirect to login page
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Session cookie exists, user is logged in, call next handler
		next(w, r)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "home.html", nil)
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tpl.ExecuteTemplate(w, "signup.html", nil)
	} else {
		// Parse form data
		fname := r.FormValue("registerFname")
		lname := r.FormValue("registerLname")
		email := r.FormValue("registerEmail")
		password := r.FormValue("registerPassword")

		// check email availability
		stmt := "select count(*) from Customers where Customers_email = ?"
		row := db.QueryRow(stmt, email)
		var count int
		err := row.Scan(&count)
		if err != nil {
			fmt.Println("Error checking email availability: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error": "internal server error"}`)
			return
		}
		if count > 0 {
			fmt.Println("Email exists, error: ", err)
			tpl.ExecuteTemplate(w, "signup.html", "Email already taken, try again!!")
			return
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Println("bcrypt err: ", err)
			tpl.ExecuteTemplate(w, "signup.html", "There was a problem registering account")
			return
		}

		_, err = db.Exec("INSERT INTO Customers(Customers_fname, Customers_lname, Customers_email, Customers_password) VALUES (?, ?, ?, ?)", fname, lname, email, string(hash))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login?success=1", http.StatusSeeOther)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tpl.ExecuteTemplate(w, "login.html", nil)
	} else {
		email := r.FormValue("loginName")
		password := r.FormValue("loginPassword")

		var dbID string
		var dbName string
		var dbEmail string
		var dbPassword string
		err := db.QueryRow("SELECT Customers_id, Customers_fname, Customers_email, Customers_password FROM Customers WHERE Customers_email=?", email).Scan(&dbID, &dbName, &dbEmail, &dbPassword)
		if err != nil {
			fmt.Println(err)
			tpl.ExecuteTemplate(w, "login.html", "There was a problem logging in, please try again!!")
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(password))
		if err != nil {
			tpl.ExecuteTemplate(w, "login.html", "Incorrect email or password, please try again!!")
			return
		}

		// Login successful, redirect to dashboard page
		expiration := time.Now().Add(24 * time.Hour) // set the cookie expiration time
		cookie := http.Cookie{Name: "user", Value: "true", Expires: expiration}
		http.SetCookie(w, &cookie)

		http.Redirect(w, r, fmt.Sprintf("/dashboard?dbID=%s&dbName=%s", dbID, dbName), http.StatusSeeOther)
	}
}

func dashHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is logged in
	cookie, err := r.Cookie("user")
	if err != nil || cookie.Value != "true" {
		// User is not logged in, redirect to login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	//SELECTING MOVIE ROWS
	movieRows, err := db.Query("SELECT * FROM Movies")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer movieRows.Close()

	// Process rows into a slice of struct or map
	var movieDetails []MoviesStruct
	for movieRows.Next() {
		var movie = MoviesStruct{}
		err := movieRows.Scan(&movie.MovieID, &movie.MovieName, &movie.MovieDesc, &movie.MoviePrice, &movie.CinemaHallID, &movie.PremierDate, &movie.ImageUrl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		movieDetails = append(movieDetails, movie)
	}
	if err = movieRows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//SELECTING CINEMA ROWS
	cinemaRows, err := db.Query("SELECT * FROM Cinema_halls")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cinemaRows.Close()

	// Process rows into a slice of struct or map
	var cinemaDetails []CinemaStruct
	for cinemaRows.Next() {
		var cinema = CinemaStruct{}
		err := cinemaRows.Scan(&cinema.CinemaID, &cinema.CinemaName, &cinema.CinemaLocation, &cinema.CinemaCapacity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cinemaDetails = append(cinemaDetails, cinema)

	}
	if err = cinemaRows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//SELECTING TICKET ROWS
	ticketRows, err := db.Query("SELECT * FROM Tickets")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer ticketRows.Close()

	// Process rows into a slice of struct or map
	var ticketDetails []TicketsStruct
	for ticketRows.Next() {
		var ticket = TicketsStruct{}
		err := ticketRows.Scan(&ticket.TicketID, &ticket.SeatNo, &ticket.Availability, &ticket.HallID, &ticket.MovieID, &ticket.CustomerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ticketDetails = append(ticketDetails, ticket)

	}
	if err = ticketRows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert cinemaDetails slice to JSON
	cinemaDetailsJSON, err := json.Marshal(cinemaDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert cinemaDetails slice to JSON
	movieDetailsJSON, err := json.Marshal(movieDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//Convert ticketDetails slice to JSON
	ticketDetailsJSON, err := json.Marshal(ticketDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	DashData := DashboardData{Movies: movieDetails, CinemaJSON: string(cinemaDetailsJSON), MoviesJSON: string(movieDetailsJSON), TicketJSON: string(ticketDetailsJSON)}

	tpl2, err := template.ParseFiles("views/screens/dashboard.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tpl2.Execute(w, DashData)

}

func payHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is logged in
	cookie, err := r.Cookie("user")
	if err != nil || cookie.Value != "true" {
		// User is not logged in, redirect to login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tpl2.ExecuteTemplate(w, "payment.html", nil)
}

func monHandler(w http.ResponseWriter, r *http.Request) {
	var seats []Seat
	err := json.NewDecoder(r.Body).Decode(&seats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, seat := range seats {
		cinemaID, err := strconv.Atoi(seat.CinemaID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		seatNumber, err := strconv.Atoi(seat.SeatNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(seat.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		movieID, err := strconv.Atoi(seat.MovieID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err = db.Exec("INSERT INTO Tickets (Tickets_seat_no, Tickets_availability, Cinema_halls_Cinema_halls_id, Movies_Movies_id, Customers_Customers_id) VALUES (?, ?, ?, ?, ?)", seatNumber, 0, cinemaID, movieID, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//w.Write([]byte("Selected seats inserted successfully"))
	json.NewEncoder(w).Encode(map[string]string{"message": "Selected seats inserted successfully"})
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is logged in
	cookie, err := r.Cookie("user")
	if err != nil || cookie.Value != "true" {
		// User is not logged in, redirect to login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	//SELECTING HISTORY ROWS
	historyRows, err := db.Query("SELECT * FROM Movie_has_customers")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer historyRows.Close()

	// Process rows into a slice of struct or map
	var HistoryDetails []HistoryStruct
	for historyRows.Next() {
		var history = HistoryStruct{}
		err := historyRows.Scan(&history.HistoryID, &history.CustomerID, &history.TicketID, &history.CinemaID, &history.MovieID, &history.TimeStamp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		HistoryDetails = append(HistoryDetails, history)
	}
	if err = historyRows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tpl2.ExecuteTemplate(w, "history.html", HistoryDetails)

}

func deleteHistoryHandler(w http.ResponseWriter, r *http.Request) {
	historyId := r.URL.Query().Get("historyId")
	_, err := db.Exec("DELETE FROM Movie_has_customers WHERE Movie_has_customers_id=?", historyId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "success"}`))
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	cookie := &http.Cookie{
		Name:   "user",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(w, cookie)

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
