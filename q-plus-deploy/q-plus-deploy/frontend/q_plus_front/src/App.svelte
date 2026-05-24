<script lang="ts">
    import ScheduleQueues from "./page/ScheduleQueues.svelte";
    import Main from "./page/Main.svelte";
    import {Link, Route, Router} from "svelte-routing";
    import Queues from "./page/Queues.svelte";


</script>

<Router>
    <Route path="/">
        <nav>
            <div>
                <Link to="/create-template/4">Создать/скопировать шаблон</Link>
            </div>
            <div>
                <Link to="/schedule-queues/20">Запланировать очереди</Link>
            </div>
            <div>
                <Link to="/queues/20">Список очередей</Link>
            </div>
        </nav>
    </Route>

    <Route path="/create-template/:courseId" let:params>
        <Main parentCourseInstanceId={+params.courseId}/>
    </Route>
    <Route path="/schedule-queues/:templateId" let:params>
        <ScheduleQueues queueTemplateId={+params.templateId}/>
    </Route>
    <Route path="queues/:templateId" let:params>
        <Queues queueTemplateId={+params.templateId}/>
    </Route>
</Router>


<!--<ScheduleQueues queueTemplate={{id: 20}}/>-->