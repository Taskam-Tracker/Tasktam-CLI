package render

// ToJalali converts a Gregorian date to the Solar Hijri (Jalali) calendar used in Iran.
// Arithmetic from the jalaali-js algorithm; exact for the 1600–3000 range.
func ToJalali(gy, gm, gd int) (jy, jm, jd int) {
	cumulative := [...]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	gy2 := gy
	if gm > 2 {
		gy2 = gy + 1
	}
	days := 355666 + 365*gy + (gy2+3)/4 - (gy2+99)/100 + (gy2+399)/400 + gd + cumulative[gm-1]
	jy = -1595 + 33*(days/12053)
	days %= 12053
	jy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}
	if days < 186 {
		return jy, 1 + days/31, 1 + days%31
	}
	return jy, 7 + (days-186)/30, 1 + (days-186)%30
}

var jalaliMonths = [...]string{
	"Farvardin", "Ordibehesht", "Khordad", "Tir", "Mordad", "Shahrivar",
	"Mehr", "Aban", "Azar", "Dey", "Bahman", "Esfand",
}
