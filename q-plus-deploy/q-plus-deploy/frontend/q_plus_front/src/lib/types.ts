// type CourseInstance = {
//     id: string;
//     name: string;
//     templates: QueueTemplate[];
// }
//
// type QueueTemplate = {
//     id: string;
//     name: string;
//     courseInstanceId: string;
// }

type IntimeFaculty = {
    id: string;
    name: string;
}

type IntimeGroup = {
    id: string;
    name: string;
}

type Lesson = {
    title: string;
    groups: IntimeGroup[];
    lessonType: string;
    startTime: number;
    endTime: number;
    date: string;
    professor: string;
    audience: string;
}

type SearchFirstLessonFilter = {
    group: IntimeGroup | undefined;
    date: string;
}

type LessonsFilter = {
    title: string;
    groups: IntimeGroup[];
    lessonType: string;
    startDate: string;
    endDate: string;
}

type ScheduleDay<T> = {
    date: string;
    items: T[];
}

type ScheduleWeek<T> = {
    startDate: string;
    endDate: string;
    days: ScheduleDay<T>[];
}