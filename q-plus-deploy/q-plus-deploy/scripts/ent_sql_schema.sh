#!/bin/bash
# install Atlas https://entgo.io/docs/getting-started/#install-atlas

atlas schema inspect -u "ent://internal/ent/schema" --dev-url "sqlite://file?mode=memory&_fk=1" --format '{{ sql . "  " }}'
