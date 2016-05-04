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
