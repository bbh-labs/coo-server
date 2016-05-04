# User
INCR nextUserID

HMSET user:[userID]
    id              (int)
    firstname       (string)
    lastname        (string)
    description     (string)
    email           (string)
    password        (string)
    blocked         (bool)
    birthdate       (date)
    imageURL        (string)
    travellingAs    (string)
    wechatNumber    (string)
    lineNumber      (string)
    facebookNumber  (string)
    skypeNumber     (string)
    whatsappNumber  (string)
    createdAt       (time)
    updatedAt       (time)

# Users with the same Interests
ZADD interest:[interest] (time) [userID]
ZADD user:[userID]:interests (time) [interest]

# Users
ZADD users (time) [userID]

# User Connections
ZADD userConnections:[userID] (time) [userID]

# Room Booking
HMSET roomBooking:[roomBookingID]
    userID         (int)
    checkinDate    (date)
    checkoutDate   (date)
    createdAt      (time)
    updatedAt      (time)

# Room Bookings
ZADD roomBookings:[userID] (time) [roomBookingID]

# LongTable
INCR nextLongTableID

HMSET longTable:[longTableID]
    id           (int)
    userID       (int)
    name         (string)
    numSeats     (int)
    openingTime  (time)
    closingTime  (time)
    createdAt    (time)
    updatedAt    (time)

# LongTables
ZADD longTables (time) [longTableID]

# LongTable Booking
INCR nextLongTableBookingID

HMSET longTableBooking:[longTableBooking]
    id           (int)
    userID       (int)
    longTableID  (int)
    seatPosition (int)
    date         (date)
    createdAt    (time)
    updatedAt    (time)

# LongTable Bookings
ZADD longTableBookings:[longTableID]:[date] (time) [longTableBookingID]
ZADD longTableBookings:[longTableID] (time) [longTableBookingID]
ZADD userLongTableBookings:[userID] (time) [longTableBookingID]
ZADD userLongTableBookings:[userID]:[date] (time) [longTableBookingID]

# Posts (e.g. offers, events, reviews)
HMSET post:[postID]
    id          (int)
    type        (string)
    userID      (int)
    title       (string)
    description (string)
    link        (string)
    imageURL    (string)
    createdAt   (time)
    updatedAt   (time)

ZADD

# Post Meta
ZADD post:[postID]:[key] [value]

# Media (e.g. Menu pdf)
HMSET media:[mediaID]
    id        (int)
    userID    (int)
    imageURL  (string)
    createdAt (time)
    updatedAt (time)

ZADD

Add new reviews, update menu, upload new image to gallery, offers, events

