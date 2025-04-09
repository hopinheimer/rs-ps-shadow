use std::marker::PhantomData;
use tracing::{span, Subscriber};
use tracing_subscriber::{layer::*, registry::LookupSpan, Layer};

pub struct NodeIdFieldLayer<S> {
    pub node_id: u64,
    _subscriber: PhantomData<S>,
}

impl<S> NodeIdFieldLayer<S> {
    pub fn new(node_id: u64) -> Self {
        Self {
            node_id,
            _subscriber: PhantomData,
        }
    }
}

impl<S> Layer<S> for NodeIdFieldLayer<S>
where
    S: Subscriber + for<'a> LookupSpan<'a>,
{
    fn on_new_span(
        &self,
        attrs: &span::Attributes<'_>,
        id: &span::Id,
        ctx: Context<'_, S>,
    ) {
        let span = ctx.span(id).expect("Span not found, this is a bug");
        
        let mut extensions = span.extensions_mut();
        extensions.insert(NodeIdField(self.node_id.clone()));
    }
}

#[derive(Debug)]
struct NodeIdField(u64);
