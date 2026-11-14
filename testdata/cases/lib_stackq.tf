/require stack-q.tf
/def r_pop = /echo [$(/pop %*)]
/def q_pop = /let _r=$(/pop %*)%; /echo {%{_r}}
/r_pop 
/r_pop stack
/r_pop nosuchstack
/q_pop 
/def r_dequeue = /echo [$(/dequeue %*)]
/def q_dequeue = /let _r=$(/dequeue %*)%; /echo {%{_r}}
/r_dequeue 
/r_dequeue queue
/q_dequeue 
/set stack=
/set queue=
/push a
/push b
/push c
/test E(strcat("[", stack, "]"))
/r_pop
/r_pop
/test E(strcat("[", stack, "]"))
/r_pop
/r_pop
/r_pop
/test E(strcat("[", stack, "]"))
/push "x y"
/push 3
/test E(strcat("[", stack, "]"))
/push one s2
/push two s2
/test E(strcat("[", s2, "]"))
/r_pop s2
/test E(strcat("[", s2, "]"))
/enqueue a
/enqueue b
/enqueue c
/test E(strcat("[", queue, "]"))
/r_dequeue
/test E(strcat("[", queue, "]"))
/r_dequeue
/r_dequeue
/r_dequeue
/enqueue 1 q2
/enqueue 2 q2
/enqueue 3 q2
/test E(strcat("[", q2, "]"))
/r_dequeue q2
/r_dequeue q2
/test E(strcat("[", q2, "]"))
/push a
/enqueue b
/test E(strcat("[", stack, "]", "[", queue, "]"))
