
```
create table kube_pod_event
(
    id               bigint(20)   not null auto_increment primary key comment 'event primary key',
    name             varchar(64)  not null default '' comment 'event name',
    namespace        varchar(64)  not null default '' comment 'event namespace',
    type             varchar(64)  not null default '' comment 'event type Warning or Normal',
    reason           varchar(64)  not null default '' comment 'event reason',
    message          text  not null  comment 'event message' ,
    kind             varchar(64)  not null default '' comment 'event kind' ,
    trigger_time     DATETIME DEFAULT CURRENT_TIMESTAMP comment 'event happens time',
    create_time 	 DATETIME DEFAULT CURRENT_TIMESTAMP,
    index pod_name_index (name)
) ENGINE = InnoDB default CHARSET = utf8 comment ='Event info tables';
 ```