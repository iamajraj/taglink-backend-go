
# TagLink Backend Project using Golang

You can think this as a simple version of LinkTree  
TagLink service backend created using GORM for Database handling and used CHI for router. This taglink backend has endpoints for user, taglinks, slots. With these endpoints one can create user, and then user's taglinks those taglinks are special links of a user in which he will have one or more slots. So when user visits his generated taglink he will see the all the slot links that he created.
## API endpoints

- `Get`   `/users`
- `Get`   `/taglinks`
- `Get`   `/slots`
- `Post`  `/users`
- `Post`  `/taglinks`
- `Post`  `/slots`
- `Post`  `/set-active-slot`
