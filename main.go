package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	Id        int       `json:"id" gorm:"primaryKey;autoIncrement:true"`
	Name      string    `json:"name" validate:"required"`
	Email     string    `json:"email" validate:"required"`
	TagLink   []TagLink `gorm:"foreignKey:UserId"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TagLink struct {
	Id           int       `json:"id" gorm:"primaryKey;autoIncrement:true"`
	TagId        string    `json:"tag_id" gorm:"unique"`
	UserId       int       `json:"user_id" validate:"required" gorm:"references:Id"`
	Slots        []Slot    `gorm:"foreignKey:TagLinkId"`
	ActiveSlotID int       `json:"active_slot_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Slot struct {
	Id        int       `json:"id" gorm:"primaryKey;autoIncrement:true"`
	Name      string    `json:"name"`
	Link      string    `json:"link"`
	TagLinkId int       `json:"tag_link_id" gorm:"references:Id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AsJson = map[string]interface{}

var validate validator.Validate

func main() {
	validate = *validator.New()
	dsn := "host=localhost user=postgres password=root dbname=taglink port=5432"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	// db.Migrator().DropTable(&Slot{}, &TagLink{}, &User{})
	if err := db.AutoMigrate(&User{}, &TagLink{}, &Slot{}); err != nil {
		log.Fatalln("Failed to migrate tables")
		return
	} else {
		fmt.Println("Tables are migrated")
	}

	if err != nil {
		log.Fatalln("Failed to connect to database")
		return
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
		var users []User
		db.Model(&User{}).Preload("TagLink").Preload("TagLink.Slots").Find(&users)
		sendJSON(users, &w)
	})

	r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
		var user User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			sendErrorMsg("Can't parse the body", &w)
			return
		}

		if err := validate.Struct(&user); err != nil {
			sendErrorMsg("validation failed", &w)
			return
		}

		db.Create(&user)

		sendJSON(AsJson{
			"message": "Success",
		}, &w)
	})

	r.Post("/taglinks", func(w http.ResponseWriter, r *http.Request) {
		tagCreation := struct {
			UserId int    `json:"user_id" validate:"required"`
			TagId  string `json:"tag_id" validate:"required"`
		}{}
		if err := json.NewDecoder(r.Body).Decode(&tagCreation); err != nil {
			sendErrorMsg("Failed to parse body", &w)
			return
		}

		if err := validate.Struct(&tagCreation); err != nil {
			sendErrorMsg("Validation failed", &w)
			return
		}

		var user User
		if err := db.Where("id=?", tagCreation.UserId).First(&user).Error; err != nil {
			sendErrorMsg("User not found", &w)
			return
		}

		var tagIdExists bool
		db.Model(&TagLink{}).Select("count(*) > 0").Where("tag_id=?", tagCreation.TagId).Find(&tagIdExists)
		if tagIdExists {
			sendErrorMsg("Given tagId already claimed", &w)
			return
		}

		var tagLink TagLink
		tagLink.TagId = tagCreation.TagId
		tagLink.UserId = tagCreation.UserId

		fmt.Println(tagCreation)

		if err := db.Create(&tagLink).Error; err != nil {
			sendErrorMsg("Failed to create taglink, please try again", &w)
			return
		}

		sendJSON(tagLink, &w)
	})

	r.Get("/taglinks", func(w http.ResponseWriter, r *http.Request) {
		var tags []TagLink
		db.Model(&TagLink{}).Preload("Slots").Find(&tags)
		sendJSON(tags, &w)
	})

	r.Post("/slots", func(w http.ResponseWriter, r *http.Request) {
		slotCreation := struct {
			Name      string `json:"name" validate:"required"`
			Link      string `json:"link" validate:"required"`
			TagLinkId int    `json:"tag_link_id" validate:"required"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&slotCreation); err != nil {
			sendErrorMsg("Failed to parse body", &w)
			return
		}

		if err := validate.Struct(&slotCreation); err != nil {
			sendErrorMsg("Validation failed", &w)
			return
		}

		var tagLink TagLink
		if err := db.Where("id=?", slotCreation.TagLinkId).First(&tagLink).Error; err != nil {
			sendErrorMsg("Given TagLink Id not exist", &w)
			return
		}

		var slot Slot
		slot.Name = slotCreation.Name
		slot.Link = slotCreation.Link
		slot.TagLinkId = slotCreation.TagLinkId

		if err := db.Model(&Slot{}).Create(&slot).Error; err != nil {
			sendErrorMsg("Failed to create taglink, please try again", &w)
			return
		}

		if tagLink.ActiveSlotID == 0 {
			tagLink.ActiveSlotID = slot.Id
		}

		db.Save(&tagLink)

		sendJSON(slot, &w)
	})

	r.Get("/slots", func(w http.ResponseWriter, r *http.Request) {
		var slots []Slot
		db.Model(&Slot{}).Find(&slots)
		sendJSON(slots, &w)
	})

	r.Post("/set-active-slot", func(w http.ResponseWriter, r *http.Request) {
		activeSlotBody := struct {
			TagLinkId int `json:"tag_link_id" validate:"required"`
			SlotId    int `json:"slot_id" validate:"required"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&activeSlotBody); err != nil {
			sendErrorMsg("Failed to parse body", &w)
			return
		}

		if err := validate.Struct(&activeSlotBody); err != nil {
			sendErrorMsg("Validation failed", &w)
			return
		}

		var tagLink TagLink
		if err := db.Where("id=?", activeSlotBody.TagLinkId).Preload("Slots").First(&tagLink).Error; err != nil {
			sendErrorMsg("Given TagLink Id doesn't exist", &w)
			return
		}

		var isSlotExists bool
		for _, slot := range tagLink.Slots {
			if slot.Id == activeSlotBody.SlotId {
				isSlotExists = true
			}
		}
		if !isSlotExists {
			sendErrorMsg("Given Slot Id doesn't exist", &w)
			return
		}
		tagLink.ActiveSlotID = activeSlotBody.SlotId
		if err := db.Save(&tagLink).Error; err != nil {
			sendErrorMsg("Failed to set active slot id, please try again", &w)
			return
		}

		sendJSON(tagLink, &w)
	})

	fmt.Println("Server started on port :3000")
	http.ListenAndServe("localhost:3000", r)
}

func sendErrorMsg(message string, w *http.ResponseWriter) {
	(*w).Write([]byte(message))
}

func sendJSON(json_de interface{}, w *http.ResponseWriter) {
	json_en, err := json.Marshal(json_de)
	(*w).Header().Add("Content-Type", "application/json")
	if err != nil {
		sendErrorMsg("Can't conver to json", w)
		return
	}
	(*w).Write(json_en)
}
