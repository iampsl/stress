压测工具

SELECT lb_user_1.user_name, lb_user_1.password,agent_code FROM lb_user_1, lb_agent_user WHERE agent_id = id AND lb_user_1.account_state = 1 AND lb_agent_user.`account_state` = 1 AND alias = 'test member' LIMIT 10