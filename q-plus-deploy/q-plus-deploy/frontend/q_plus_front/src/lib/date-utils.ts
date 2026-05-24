export function getWeeksFromDays<T>(days: ScheduleDay<T>[]): ScheduleWeek<T>[] {
    const parseDate = (dateStr: string): Date => new Date(dateStr);
    const startOfWeek = (date: Date): Date => {
        const day = date.getDay();
        const diff = (day === 0 ? -6 : 1) - day;
        const start = new Date(date);
        start.setDate(date.getDate() + diff);
        return start;
    }
    const addDays = (date: Date, days: number): Date => {
        const result = new Date(date);
        result.setDate(date.getDate() + days);
        return result;
    }

    // Sort days by date
    days.sort((a, b) => parseDate(a.date).getTime() - parseDate(b.date).getTime());

    if (days.length === 0) return [];

    // Find the range of dates covered by the days array
    const firstDate = parseDate(days[0].date);
    const lastDate = parseDate(days[days.length - 1].date);

    // Calculate the range of weeks
    const firstMonday = startOfWeek(firstDate);
    const lastSunday = addDays(startOfWeek(lastDate), 6);

    // Initialize an array to hold the weeks
    const weeks: ScheduleWeek<T>[] = [];

    // Create empty weeks from the first Monday to the last Sunday
    let currentMonday = firstMonday;
    while (currentMonday <= lastSunday) {
        const currentSunday = addDays(currentMonday, 6);
        const weekDays: ScheduleDay<T>[] = [];
        for (let i = 0; i < 6; i++) {
            const dayDate = addDays(currentMonday, i);
            weekDays.push({
                date: dayDate.toISOString().split('T')[0],
                items: []
            });
        }
        weeks.push({
            startDate: currentMonday.toISOString().split('T')[0],
            endDate: currentSunday.toISOString().split('T')[0],
            days: weekDays
        });
        currentMonday = addDays(currentMonday, 7);
    }

    // Fill the weeks with the days
    for (const day of days) {
        const dayDate = parseDate(day.date);
        const weekIndex = Math.floor((dayDate.getTime() - firstMonday.getTime()) / (7 * 24 * 60 * 60 * 1000));
        const week = weeks[weekIndex];
        const dayIndex = (dayDate.getDay() + 6) % 7; // Adjust for Monday being the start of the week
        week.days[dayIndex] = day;
    }

    // Filter out empty weeks (weeks with no lessons)
    return weeks.filter(week => week.days.some(day => day.items.length > 0));
}
