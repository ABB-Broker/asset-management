CREATE TABLE IF NOT EXISTS locations (
    location_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    location_uuid VARCHAR(36),
    location_name VARCHAR(255) NOT NULL,
    description   TEXT,
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME,
    INDEX idx_locations_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS categories (
    category_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME,
    INDEX idx_categories_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS users (
    user_no     INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username    VARCHAR(255) NOT NULL,
    email       VARCHAR(255),
    password    VARCHAR(255),
    full_name   VARCHAR(255),
    phone_number VARCHAR(255),
    department  VARCHAR(255),
    position    VARCHAR(255),
    employee_id VARCHAR(255),
    role        ENUM('admin','editor','viewer') DEFAULT 'viewer',
    active      BOOLEAN DEFAULT TRUE,
    assignee_no INT UNSIGNED,
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME,
    UNIQUE INDEX idx_users_username (username),
    UNIQUE INDEX idx_users_email (email),
    UNIQUE INDEX idx_users_employee_id (employee_id),
    INDEX idx_users_deleted_at (deleted_at),
    INDEX idx_users_assignee_no (assignee_no)
);

CREATE TABLE IF NOT EXISTS sessions (
    session_no    INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    token         VARCHAR(255) NOT NULL,
    user_no       INT UNSIGNED NOT NULL,
    authenticated BOOLEAN DEFAULT FALSE,
    pending_2fa   BOOLEAN DEFAULT TRUE,
    expires_at    DATETIME,
    created_at    DATETIME,
    updated_at    DATETIME,
    deleted_at    DATETIME,
    UNIQUE INDEX idx_sessions_token (token),
    INDEX idx_sessions_deleted_at (deleted_at),
    INDEX idx_sessions_expires_at (expires_at),
    INDEX idx_sessions_user_no (user_no),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_no) REFERENCES users(user_no) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS assignees (
    assignee_no   INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    assignee_uuid VARCHAR(36),
    full_name     VARCHAR(255) NOT NULL,
    email         VARCHAR(255),
    phone_number  VARCHAR(255),
    user_no       INT UNSIGNED,
    department    VARCHAR(255),
    position      VARCHAR(255),
    employee_id   VARCHAR(255),
    company       VARCHAR(255),
    notes         TEXT,
    created_at    DATETIME,
    updated_at    DATETIME,
    deleted_at    DATETIME,
    UNIQUE INDEX idx_assignees_email (email),
    INDEX idx_assignees_deleted_at (deleted_at),
    INDEX idx_assignees_user_no (user_no),
    CONSTRAINT fk_assignees_user FOREIGN KEY (user_no) REFERENCES users(user_no) ON DELETE SET NULL
);

-- Now safe to add circular FK: users.assignee_no -> assignees
ALTER TABLE users
    ADD CONSTRAINT fk_users_assignee FOREIGN KEY (assignee_no) REFERENCES assignees(assignee_no) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS assets (
    asset_no       INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    asset_uuid     VARCHAR(36),
    name           VARCHAR(255),
    description    TEXT,
    asset_type     ENUM('fixed','movable') NOT NULL DEFAULT 'fixed',
    category_no    INT UNSIGNED NOT NULL,
    location_no    INT UNSIGNED,
    serial_number  VARCHAR(255),
    purchase_date  VARCHAR(255),
    purchase_price INT UNSIGNED,
    created_at     DATETIME,
    updated_at     DATETIME,
    deleted_at     DATETIME,
    INDEX idx_assets_deleted_at (deleted_at),
    INDEX idx_assets_category_no (category_no),
    INDEX idx_assets_location_no (location_no),
    CONSTRAINT fk_assets_category FOREIGN KEY (category_no) REFERENCES categories(category_no) ON DELETE CASCADE,
    CONSTRAINT fk_assets_location FOREIGN KEY (location_no) REFERENCES locations(location_no) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS lending_logs (
    lending_log_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    lending_uuid   VARCHAR(36),
    asset_no       INT UNSIGNED NOT NULL,
    assignee_no    INT UNSIGNED NOT NULL,
    lent_at        DATETIME,
    planned_use_at DATETIME,
    returned_at    DATETIME,
    status         ENUM('pending_signature','active','returned') DEFAULT 'pending_signature',
    notes          TEXT,
    created_at     DATETIME,
    updated_at     DATETIME,
    deleted_at     DATETIME,
    INDEX idx_lending_logs_deleted_at (deleted_at),
    INDEX idx_lending_logs_asset_no (asset_no),
    INDEX idx_lending_logs_assignee_no (assignee_no),
    CONSTRAINT fk_lending_logs_asset    FOREIGN KEY (asset_no)    REFERENCES assets(asset_no)       ON DELETE CASCADE,
    CONSTRAINT fk_lending_logs_assignee FOREIGN KEY (assignee_no) REFERENCES assignees(assignee_no) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS handover_forms (
    handover_form_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    form_uuid        VARCHAR(36),
    lending_log_no   INT UNSIGNED NOT NULL,
    form_token       VARCHAR(255),
    sent_at          DATETIME,
    signed_at        DATETIME,
    signature_data   LONGTEXT,
    status           ENUM('sent','signed','published') DEFAULT 'sent',
    receipt_path     VARCHAR(255),
    created_at       DATETIME,
    updated_at       DATETIME,
    deleted_at       DATETIME,
    UNIQUE INDEX idx_handover_forms_lending_log_no (lending_log_no),
    UNIQUE INDEX idx_handover_forms_form_token (form_token),
    INDEX idx_handover_forms_deleted_at (deleted_at),
    CONSTRAINT fk_handover_forms_lending_log FOREIGN KEY (lending_log_no) REFERENCES lending_logs(lending_log_no) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS location_photos (
    location_photo_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    location_no       INT UNSIGNED NOT NULL,
    name              VARCHAR(255),
    photo_url         VARCHAR(255),
    created_at        DATETIME,
    updated_at        DATETIME,
    deleted_at        DATETIME,
    INDEX idx_location_photos_deleted_at (deleted_at),
    INDEX idx_location_photos_location_no (location_no),
    CONSTRAINT fk_location_photos_location FOREIGN KEY (location_no) REFERENCES locations(location_no) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS asset_photos (
    asset_photo_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    asset_no       INT UNSIGNED NOT NULL,
    name           VARCHAR(255),
    photo_url      VARCHAR(255),
    created_at     DATETIME,
    updated_at     DATETIME,
    deleted_at     DATETIME,
    INDEX idx_asset_photos_deleted_at (deleted_at),
    INDEX idx_asset_photos_asset_no (asset_no),
    CONSTRAINT fk_asset_photos_asset FOREIGN KEY (asset_no) REFERENCES assets(asset_no) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS pics (
    pic_no     INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    asset_no   INT UNSIGNED NOT NULL,
    user_no    INT UNSIGNED NOT NULL,
    notes      TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    INDEX idx_pics_deleted_at (deleted_at),
    INDEX idx_pics_asset_no (asset_no),
    INDEX idx_pics_user_no (user_no),
    CONSTRAINT fk_pics_asset FOREIGN KEY (asset_no) REFERENCES assets(asset_no) ON DELETE CASCADE,
    CONSTRAINT fk_pics_user  FOREIGN KEY (user_no)  REFERENCES users(user_no)   ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS password_set_tokens (
    password_set_token_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    token                 VARCHAR(255) NOT NULL,
    user_no               INT UNSIGNED NOT NULL,
    kind                  ENUM('invite','reset') NOT NULL DEFAULT 'invite',
    used_at               DATETIME,
    expires_at            DATETIME,
    created_at            DATETIME,
    updated_at            DATETIME,
    deleted_at            DATETIME,
    UNIQUE INDEX idx_password_set_tokens_token (token),
    INDEX idx_password_set_tokens_deleted_at (deleted_at),
    INDEX idx_password_set_tokens_user_no (user_no),
    INDEX idx_password_set_tokens_expires_at (expires_at),
    CONSTRAINT fk_password_set_tokens_user FOREIGN KEY (user_no) REFERENCES users(user_no) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS email_otps (
    email_otp_no INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    code         VARCHAR(255) NOT NULL,
    user_no      INT UNSIGNED NOT NULL,
    used_at      DATETIME,
    expires_at   DATETIME,
    created_at   DATETIME,
    updated_at   DATETIME,
    deleted_at   DATETIME,
    INDEX idx_email_otps_deleted_at (deleted_at),
    INDEX idx_email_otps_user_no (user_no),
    INDEX idx_email_otps_expires_at (expires_at),
    CONSTRAINT fk_email_otps_user FOREIGN KEY (user_no) REFERENCES users(user_no) ON DELETE CASCADE
);