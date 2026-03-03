use std::time::{Duration, SystemTime};

pub fn renewal_delay(now: SystemTime, expires_at: SystemTime) -> Duration {
    let Ok(remaining) = expires_at.duration_since(now) else {
        return Duration::from_secs(0);
    };

    let target = Duration::from_secs_f64(remaining.as_secs_f64() * 0.8);
    if target.is_zero() {
        Duration::from_secs(1)
    } else {
        target
    }
}

#[cfg(test)]
mod tests {
    use super::renewal_delay;
    use std::time::{Duration, SystemTime};

    #[test]
    fn schedules_at_eighty_percent_of_remaining_lifetime() {
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(100);
        let expires_at = now + Duration::from_secs(100);
        assert_eq!(renewal_delay(now, expires_at), Duration::from_secs(80));
    }

    #[test]
    fn returns_zero_when_certificate_is_already_expired() {
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(200);
        let expired = SystemTime::UNIX_EPOCH + Duration::from_secs(100);
        assert_eq!(renewal_delay(now, expired), Duration::from_secs(0));
    }

    #[test]
    fn enforces_minimum_one_second_when_expiry_equals_now() {
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(100);
        let expires_at = now;
        assert_eq!(renewal_delay(now, expires_at), Duration::from_secs(1));
    }
}
