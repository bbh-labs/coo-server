# User
INCR nextUserID

HMSET user:[userID]
    firstname       (string)
    lastname        (string)
    description     (string)
    email           (string)
    password        (string)
    blocked         (bool)
    birthdate       (date)
    image_url       (string)
    travelling_as   (string)
    wechat_number   (string)
    line_number     (string)
    facebook_number (string)
    skype_number    (string)
    whatsapp_number (string)
    created_at      (time)
    updated_at      (time)

# User with the same Interests
ZADD interest:[interest] (time) [userID]

# Users
ZADD users (time) [userID]

# User-to-User
ZADD users:[userID] (time) [userID]

# Room Booking
HMSET roomBooking:[roomBookingID]
    user_id         (int)
    checkin_date    (date)
    checkout_date   (date)
    created_at      (time)
    updated_at      (time)

# Room Bookings
ZADD roomBookings:[userID] (time) [roomBookingID]

# LongTable
INCR nextLongTableID

HMSET longTable:[longTableID]
    name         (string)
    num_seats    (int)
    opening_time (time)
    closing_time (time)
    created_at   (time)
    updated_at   (time)

# LongTables
ZADD longTables (time) [longTableID]

# LongTable Booking
INCR nextLongTableBookingID

HMSET longTableBooking
    user_id       (int)
    seat_position (int)
    created_at    (time)
    updated_at    (time)

# LongTable Bookings
ZADD longTableBookings (time) [longTableBookingID]
