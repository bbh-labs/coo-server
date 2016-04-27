# User
INCR next_user_id

HMSET user:[user_id]
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
ZADD interest:[interest] (time) [user_id]

# Users
ZADD users (time) [user_id]

# User-to-User
ZADD users:[user_id] (time) [user_id]

# Room Booking
HMSET room_booking:[room_booking_id]
    user_id         (int)
    checkin_date    (date)
    checkout_date   (date)
    created_at      (time)
    updated_at      (time)

# Room Bookings
ZADD room_bookings:[user_id] (time) [room_booking_id]

# LongTable
INCR next_longtable_id

HMSET longtable:[longtable_id]
    name         (string)
    num_seats    (int)
    opening_time (time)
    closing_time (time)
    created_at   (time)
    updated_at   (time)

# LongTables
ZADD longtables (time) [longtable_id]

# LongTable Booking
HMSET longtable_booking
    seat_position (int)
    created_at    (time)
    updated_at    (time)
