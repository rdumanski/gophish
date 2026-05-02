-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN modified_date DATETIME;

UPDATE results
    SET `modified_date`= (
        SELECT max(events.time) FROM events
        WHERE events.email=results.email
        AND events.campaign_id=results.campaign_id
    );
