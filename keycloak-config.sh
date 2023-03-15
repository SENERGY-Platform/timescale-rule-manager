#!/bin/bash
/opt/keycloak/bin/kcadm.sh config credentials --server http://keycloak:8080 --realm master --user admin --password admin
/opt/keycloak/bin/kcadm.sh create clients -r master --server http://keycloak:8080 \
-s clientId=myapp \
-s enabled=true \
-s clientAuthenticatorType=client-secret \
-s secret=d0b8122f-8dfb-46b7-b68a-f5cc4e25d000 \
-s serviceAccountsEnabled=true
/opt/keycloak/bin/kcadm.sh add-roles --uusername service-account-myapp --rolename admin -r master
/opt/keycloak/bin/kcadm.sh create users -r master -s username=testuser -s enabled=true
/opt/keycloak/bin/kcadm.sh create users -r master -s username=testuser2 -s enabled=true

