<script lang="ts">
    import LessonCard from "./LessonCard.svelte";

    let {lesson, group = $bindable()}: {
        lesson: Lesson,
        group: Lesson[]
    } = $props();

    let selected = $state(true)
    // updateGroup()

    const onclick = () => {
        onSelect()
    }

    const onkeydown = (e: KeyboardEvent) => {
        if (e.key === "Enter" || e.key === " ") {
            onSelect()
        }
    }

    function onSelect() {
        selected = !selected;
        updateGroup();
    }

    function updateGroup() {
        if (selected) {
            group.push(lesson);
        } else {
            group = group.filter(l => {
                // TODO wtf, svelte?? I can't compare objects directly
                return l.date !== lesson.date ||
                    l.startTime !== lesson.startTime ||
                    l.endTime !== lesson.endTime ||
                    l.title !== lesson.title
            });
        }
    }
</script>

<span role="button" tabindex="0" {onclick} {onkeydown}>
    <LessonCard {lesson} {selected}/>
</span>
