# event-manager

I made this to keep track of events in one place. You can make an event, see all your events, change one, or delete it, and people can sign up to come.

To add an event you just give it a title and a date, and you can add where it is and what it is about if you want. You can search your events too, by title or place. On each event you can see who signed up, and a person signs up with their name and email.

Your events are saved in a file called events.json, so they are still there when you stop the app and start it again. There is no database to set up.

To run it, you need Go, then type go run . and open localhost:8080 in your browser. I tested it with go test ./... and it passed.
