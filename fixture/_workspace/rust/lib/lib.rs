//! Number formatting helpers.

/// Formats n as a greeting suffix.
pub fn counter(n: u64) -> String {
    let mut buf = itoa::Buffer::new();
    format!("visitor #{}", buf.format(n))
}

#[cfg(test)]
mod tests {
    #[test]
    fn formats() {
        assert_eq!(super::counter(7), "visitor #7");
    }
}
